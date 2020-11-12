package genkai

import "github.com/gin-gonic/gin"

func (k *Kai) GinInstall(engine *gin.Engine, customPath ...string) {
	rp := "__genkai_endpoint"
	if len(customPath) == 1 {
		rp = customPath[0]
	}
	engine.POST(rp, k.GinHandler)
}

func (k *Kai) GinHandler(g *gin.Context) {
	payload := &RequestPayload{}
	if isError(200, g.BindJSON(payload), g) {
		return
	}

	ctx := &Context{
		Session: g.GetHeader("genkai-session"),
		Request: payload,
	}

	resultPayload, err := k.Execute(ctx)
	if isError(200, err, g) {
		return
	}

	g.JSON(200, resultPayload)
}

