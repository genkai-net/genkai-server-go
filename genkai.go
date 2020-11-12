package genkai

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"reflect"
)


type Kai struct {
	f map[string]interface{}
}

type Context struct {
	Session string
	Request *RequestPayload
}

type I = interface {}

type ContextFunc = func(ctx *Context) I

type RequestPayload struct {
	Parameters   []interface{} `json:"p" binding:"required"`
	FunctionName string        `json:"fn" binding:"required"`
}

type ResultPayload struct {
	Returns interface{} `json:"r"`
	Error   interface{} `json:"e"`
}


func New() *Kai {

	return &Kai{
		f: map[string]interface{}{},
	}
}

func (k *Kai) Func(name string, c interface{}) {
	f := reflect.ValueOf(c)
	if f.Type().NumOut() == 0 {
		panic(fmt.Errorf("function '%v' requires to return an error variable", name))
	}
	lastparam := f.Type().Out(f.Type().NumOut() - 1)
	if lastparam.String() != "error" {
		panic(fmt.Errorf("function '%v' does not have it's last error parameter as type 'error'", name))
	}
	k.f[name] = c
}

var _emptyContext = &Context{}
func fnHasContext(f reflect.Value) bool {
	return f.Type().In(0) == reflect.TypeOf(_emptyContext)
}

func (k *Kai) Execute(ctx *Context) (*ResultPayload, error) {
	fn, exists := k.f[ctx.Request.FunctionName]
	if !exists {
		return nil, fmt.Errorf("function '%v' does not exist", ctx.Request.FunctionName)
	}

	//mainFn := fn(ctx)
	mainFn_v := reflect.ValueOf(fn)
	if fnHasContext(mainFn_v) {
		ctx.Request.Parameters = append([]interface{}{ctx}, ctx.Request.Parameters...)
	}

	if len(ctx.Request.Parameters) != mainFn_v.Type().NumIn() {
		return nil, fmt.Errorf("Function accepts %v params, provided %v", mainFn_v.Type().NumIn(), len(ctx.Request.Parameters))
	}
	in := make([]reflect.Value, len(ctx.Request.Parameters))
	for k, param := range ctx.Request.Parameters {
		in[k] = reflect.ValueOf(param)
	}

	_r := mainFn_v.Call(in)
	resultPayload := &ResultPayload{}

	result := []interface{}{}
	for _, value := range _r {
		result = append(result, value.Interface())
	}

	if mainFn_v.Type().NumOut() == 1 {
		if result[0] != nil {
			resultPayload.Error = fmt.Sprintf("%v", result[0])
		}
	} else {
		resultPayload.Returns = result[:len(result)-1]
		if result[len(result)-1] != nil {
			resultPayload.Error = fmt.Sprintf("%v", result[len(result)-1])
		}
	}

	return resultPayload, nil
}

func isError(code int, err error, g *gin.Context) bool {
	if err == nil {
		return false
	}
	g.JSON(code, gin.H{"e": fmt.Sprintf("%v", err)})
	return true
}
