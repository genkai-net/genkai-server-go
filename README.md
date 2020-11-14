# Genkai Go Server

Genkai is a dead simple function adapter between go and any language. It uses traditional HTTP for maximum 
adaptability with support for other transports coming soon.

### Installation
```bash
$ go get github.com/genkai-net/genkai-server-go
```

### Usage
Genkai as of now only supports plugging in to a GIN http server.

```go

func main() {
    engine := gin.Default()
    
	kit := genkai.New()
	kit.Func("join", func(a, b string) string {
		return strings.Join(strs, ":")
	})

	kit.GinInstall(engine)
	panic(engine.Run("localhost:9302"))
}
```
Genkai installs itself alongside a GIN server, with a `join` function exposed.

To use on another language (ex. JS) using https://github.com/genkai-net/genkai-client-js
```js
import {GenkaiClient, GenkaiClientJSON} from "genkai-client";

let app = GenkaiClient("http://localhost:9302");
let response = await app.join("hello", "world");
console.log(response);
```
Would result into an output of
```js
hello:world
```

### Contextual functions
The sample above is an example of an non-contextual function. Meaning the function has no way for differentiating
who calls the function. Fortunately, Genkai comes with session tracking out of the box by prepending `ctx *genkai.Context`
to the function's parameters.

```go
	kit.Func("join", func(ctx *genkai.Context, a, b string) string {
            j := strings.Join(strs, ":")
            return fmt.Sprintf("$v's result: %v", ctx.Session, j)
	})
```

Running the JS snippet above would render
```
__genkai_endpointomo5k5llowcrsxq1kumvg's result: hello:world
```

The session ID persistence depends on the client library, in `genkai-client-js` the session ID is stored in `SessionStorage`.
As such, it persists browser refreshes until the tab is closed.

### Errors
Since most go functions comes with errors alongside the return value, Genkai supports these out of the box.
```go
	kit.Func("login", func(ctx *genkai.Context, username, password string) (string, error) {
            if username != "myuser" && password != "secret" {
            	return "", fmt.Errorf("Invalid credentials")
            }
            
            return "account_secret_token", nil
	})
```


Here are some various possible errors that would arise from using this function.
```
let app = GenkaiClient("http://localhost:9302");
let token = await app.login("myuser", "wrong_secret");
>> Throws: Error: Invalid credentials

let token = await app.login("myuser");
>> Throws: Error: Function accepts 2 params <string, string>, provided 1

let token = await app.rogin("myuser", "secret");
>> Throws: Error: function 'rogin' does not exist

let token = await app.login("myuser", "secret");
        \-> "account_secret_token"
```

### JSONMode
If your function parameters requires far a more complex input outside of the usual strings and numbers, Genkai
comes with a struct binding support similar to GIN's `ctx.BindJSON`.

```go
    type LoginPayload struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }
    kit.Func("login", func(ctx *genkai.Context) (string, error) {
        payload := LoginPayload{}
        err := ctx.BindJSON(&payload)
        if err != nil {
            return "", err	
        }

        if payload.username != "myuser" && payload.password != "secret" {
            return "", fmt.Errorf("Invalid credentials")
        }
        
        return "account_secret_token", nil
    })
```
Accessing this type of function requires `GenkaiClientJSON` instead of `GenkaiClient`, calling this function on the vanilla
client usually throws the `Error: Function accepts 0 params <*genkai.Context>, provided *` error.
```
let app = GenkaiClientJSON("http://localhost:9302");
let token = await app.login({
    username: "myuser",
    password: "secret"
});
"account_secret_token"
```
*Restrictions*
* A JSONMode function **must** only have `*genkai.Context` as it's only parameter.
* You can pass any JS object as long as go's `encoding/json` is capable of unmarshalling it.

### Structs
Genkai supports exposing struct methods out of the box same rules apply for the functions within the struct.
```go

func main() {
	manager_instance := &Manager{
		Store:    map[string]string{},
		Sessions: map[string]string{},
		Pipe:     map[string]chan string{},
	}

	engine := gin.Default()
	c := cors.DefaultConfig()
	c.AllowAllOrigins = true
	c.AllowHeaders = append(c.AllowHeaders, "genkai-session")
	engine.Use(cors.New(c))

	kit := genkai.New()
	kit.Struct("man", manager_instance)
	kit.GinInstall(engine)

	panic(engine.Run("localhost:9302"))
}

type Manager struct {
	Store    map[string]string
	Sessions map[string]string
	Pipe     map[string]chan string
}

func (m *Manager) Register(username string) error {
	m.Store[username] = ""
	m.Pipe[username] = make(chan string, 5)
	return nil
}

func (m *Manager) Login(ctx *genkai.Context, username string) error {
	_, exists := m.Store[username]
	if !exists {
		return fmt.Errorf("user not found")
	}
	m.Sessions[ctx.Session] = username
	return nil
}

func (m *Manager) GetStore(ctx *genkai.Context) (string, error) {
	value, exists := m.Store[m.Sessions[ctx.Session]]
	if !exists {
		return "", fmt.Errorf("session not found")
	}
	return value, nil
}

func (m *Manager) SetStore(ctx *genkai.Context, value string) error {
	user, exists := m.Sessions[ctx.Session]
	if !exists {
		return fmt.Errorf("session not found")
	}

	_, exists = m.Store[user]
	if !exists {
		return fmt.Errorf("user not found")
	}

	m.Store[user] = value
	return nil
}

func (m *Manager) Details(ctx *genkai.Context) (gin.H, error) {
	user := m.Sessions[ctx.Session]
	value, exists := m.Store[user]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}
	return gin.H{
		"session": ctx.Session,
		"user":    user,
		"store":   value,
	}, nil
}

func (m *Manager) Push(ctx *genkai.Context, value string) error {
	pipe, exists := m.Pipe[m.Sessions[ctx.Session]]
	if !exists {
		return fmt.Errorf("session not found")
	}
	pipe <- value
	return nil
}

func (m *Manager) Pop(ctx *genkai.Context) (string, error) {
	pipe, exists := m.Pipe[m.Sessions[ctx.Session]]
	if !exists {
		return "", fmt.Errorf("session not found")
	}

	return <-pipe, nil
}
```

JS usage is almost the same as calling a function but with a `$` separator to distinguish the struct name and the function name.
In this case, the struct is exposed as `man` due to the `kit.Struct("man", manager_instance)` declaration.
```
let app = GenkaiClient("http://localhost:9302");
await app.man$Register("noku"); // Call func (m *Manager) Register(string) error 
await app.man$Login("noku");
await app.man$GetStore();
 \-> ""
await app.man$SetStore("Hello world!")
await app.man$GetStore();
 \-> "Hello world!"
await app.man$Details();
 \-> {session: "__genkai_endpoint...", user: "noku", store: "Hello world!"}
```