package main

import (
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"genkai"
)

type Manager struct {
	Store    map[string]string
	Sessions map[string]string
	Pipe     map[string]chan string
}

func (m *Manager) Register(username string) error {
	m.Store[username] = ""
	m.Pipe[username] = make(chan string)
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

func (m *Manager) GetStore(ctx *genkai.Context) (string, error) {
	value, exists := m.Store[m.Sessions[ctx.Session]]
	if !exists {
		return "", fmt.Errorf("session not found")
	}
	return value, nil
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

func main() {
	man := &Manager{
		Store:    map[string]string{},
		Sessions: map[string]string{},
		Pipe:     map[string]chan string{},
	}
	kit := genkai.NewKit()

	kit.Func("ping", func(ctx *genkai.Context) (interface{}, error) {
		return gin.H{
			"session": ctx.Session,
			"manager": man,
		}, nil
	})

	kit.Func("register", man.Register)
	kit.Func("login", man.Login)
	kit.Func("getStore", man.GetStore)
	kit.Func("setStore", man.SetStore)
	kit.Func("details", man.Details)
	kit.Func("push", man.Push)
	kit.Func("pop", man.Pop)

	engine := gin.Default()
	c := cors.DefaultConfig()
	c.AllowAllOrigins = true
	c.AllowHeaders = append(c.AllowHeaders, "genkai-session")
	engine.Use(cors.New(c))

	kit.GinInstall(engine)

	panic(engine.Run("localhost:9302"))
}
