package genkai

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"reflect"
	"strings"
)

type Kai struct {
	funcs   map[string]reflect.Value
	structs map[string]reflect.Value
}

type Context struct {
	Session string
	Request *RequestPayload
	Ctx     context.Context
}

type I = interface{}

type ContextFunc = func(ctx *Context) I

type RequestPayload struct {
	ID           string        `json:"id"`
	Parameters   []interface{} `json:"p"`
	FunctionName string        `json:"fn" binding:"required"`
	JSONData     string        `json:"json"`
}

type ResultPayload struct {
	ID      string      `json:"id,omitempty"`
	Returns interface{} `json:"r"`
	Error   interface{} `json:"e"`
}

func New() *Kai {

	return &Kai{
		funcs:   map[string]reflect.Value{},
		structs: map[string]reflect.Value{},
	}
}

func (k *Kai) Func(name string, c interface{}) {
	f := reflect.ValueOf(c)

	if f.Kind() != reflect.Func {
		panic(fmt.Errorf("item passed as '%v' is not a func", name))
	}

	k.funcs[name] = f
}

func (k *Kai) Struct(name string, s interface{}) {
	f := reflect.ValueOf(s)
	if f.Kind().String() == "ptr" {
		if f.Elem().Kind() != reflect.Struct {
			panic(fmt.Errorf("item passed as '%v' is not a struct", name))
		}
	} else {
		if f.Kind() != reflect.Struct {
			panic(fmt.Errorf("item passed as '%v' is not a struct", name))
		}
	}

	k.structs[name] = f
}

var _emptyContext = &Context{}

func fnHasContext(f reflect.Value) bool {
	return f.Type().In(0) == reflect.TypeOf(_emptyContext)
}

func (k *Kai) Execute(ctx *Context) (*ResultPayload, error) {
	var mainFn_v reflect.Value
	var exists bool

	if strings.Contains(ctx.Request.FunctionName, "$") {
		elements := strings.Split(ctx.Request.FunctionName, "$")
		if len(elements) != 2 {
			return nil, fmt.Errorf("struct access is only limited to top level")
		}
		var s reflect.Value
		s, exists = k.structs[elements[0]]
		if !exists {
			return nil, fmt.Errorf("struct '%v' does not exist", elements[0])
		}

		mainFn_v = s.MethodByName(elements[1])
		if !mainFn_v.IsValid() {
			return nil, fmt.Errorf("method '%v' from struct '%v' does not exist", elements[1], elements[0])
		}
	} else {
		mainFn_v, exists = k.funcs[ctx.Request.FunctionName]
		if !exists {
			return nil, fmt.Errorf("function '%v' does not exist", ctx.Request.FunctionName)
		}
	}

	hasCtx := fnHasContext(mainFn_v)
	inLength := len(ctx.Request.Parameters)
	funcInLength := mainFn_v.Type().NumIn()
	if hasCtx {
		ctx.Request.Parameters = append([]interface{}{ctx}, ctx.Request.Parameters...)
	}

	required := funcInLength
	if hasCtx {
		required -= 1
	}

	if ctx.Request.JSONData != "" && inLength > 0 {
		return nil, fmt.Errorf("Passed request contains both JSONMode and normal parameters")
	}

	if ctx.Request.JSONData != "" && !hasCtx {
		return nil, fmt.Errorf("Function '%v' is treated as JSONMode but no context has been requested in the function, please use the non-JSON client", ctx.Request.FunctionName)
	}

	if ctx.Request.JSONData != "" && funcInLength != 1 {
		return nil, fmt.Errorf("Function '%v' is not JSONMode", ctx.Request.FunctionName)
	}

	if inLength != required {
		reqParams := []string{}
		for i := 0; i < mainFn_v.Type().NumIn(); i++ {
			reqParams = append(reqParams, mainFn_v.Type().In(i).String())
		}
		return nil, fmt.Errorf("Function accepts %v params <%v>, provided %v", required, strings.Join(reqParams, ", "), inLength)
	}

	in := make([]reflect.Value, len(ctx.Request.Parameters))
	for i, param := range ctx.Request.Parameters {
		val := reflect.ValueOf(param)

		if !val.IsNil() && val.Type().Kind() == reflect.Slice {
			var err error
			val, err = k.coerceSliceType(val)
			if err != nil {
				return nil, err
			}
		}

		in[i] = val

		if mainFn_v.Type().In(i).String() != in[i].Type().String() {
			return nil, fmt.Errorf("Parameter '%v' mismatch: supplied '<%v:%v>', expected '<%v>'",
				i,
				in[i].Interface(),
				in[i].Type().String(),
				mainFn_v.Type().In(i).String(),
			)
		}
	}
	// Function call happens here
	_r := mainFn_v.Call(in)
	resultPayload := &ResultPayload{ID: ctx.Request.ID}

	result := []interface{}{}
	for _, value := range _r {
		result = append(result, value.Interface())
	}

	hasError := false
	lastParam := mainFn_v.Type().Out(mainFn_v.Type().NumOut() - 1)
	if lastParam.String() == "error" {
		hasError = true
	}

	if mainFn_v.Type().NumOut() == 1 {
		if result[0] != nil && hasError {
			resultPayload.Error = fmt.Sprintf("%v", result[0])
		} else {
			resultPayload.Returns = result
		}
	} else {
		if hasError {
			resultPayload.Returns = result[:len(result)-1]
			if result[len(result)-1] != nil {
				resultPayload.Error = fmt.Sprintf("%v", result[len(result)-1])
			}
		} else {
			resultPayload.Returns = result
		}
	}

	return resultPayload, nil
}

func (k *Kai) coerceSliceType(val reflect.Value) (reflect.Value, error) {
	if val.Len() == 0 {
		return val, nil
	}

	if _, ok := val.Index(0).Interface().(string); ok {
		newVal := []string{}
		for i := 0; i < val.Len(); i++ {
			converted, ok := val.Index(i).Interface().(string)
			if !ok {
				return reflect.Value{}, fmt.Errorf("array parameter does not contain uniform types: 'string'")
			}
			newVal = append(newVal, converted)
		}
		val = reflect.ValueOf(newVal)

	} else if _, ok := val.Index(0).Interface().(bool); ok {
		newVal := []bool{}
		for i := 0; i < val.Len(); i++ {
			converted, ok := val.Index(i).Interface().(bool)
			if !ok {
				return reflect.Value{}, fmt.Errorf("array parameter does not contain uniform types: 'bool'")
			}
			newVal = append(newVal, converted)
		}
		val = reflect.ValueOf(newVal)

	} else if _, ok := val.Index(0).Interface().(float64); ok {
		newVal := []float64{}
		for i := 0; i < val.Len(); i++ {
			converted, ok := val.Index(i).Interface().(float64)
			if !ok {
				return reflect.Value{}, fmt.Errorf("array parameter does not contain uniform types: 'float64'")
			}
			newVal = append(newVal, converted)
		}
		val = reflect.ValueOf(newVal)
	} else {
		return reflect.Value{}, fmt.Errorf("parameter contains an array with an unsupported type '%v'", val.Index(0).Interface())
	}
	return val, nil
}

var MissingJSONError = fmt.Errorf("The function is JSONMode but no JSON was supplied")

func (c *Context) BindJSON(v interface{}) error {
	if c.Request.JSONData == "" {
		return MissingJSONError
	}
	return json.Unmarshal([]byte(c.Request.JSONData), v)
}

func isError(code int, err error, g *gin.Context) bool {
	if err == nil {
		return false
	}
	g.JSON(code, gin.H{"e": fmt.Sprintf("%v", err)})
	return true
}
