package grit

import "net/http"

type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request
}

func (c *Context) JSON(data interface{}) {
	RenderJSON(c.Writer, data)
}

func (c *Context) Text(text string) {
	RenderText(c.Writer, text)
}
