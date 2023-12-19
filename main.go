package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	e := echo.New()
	e.Use(middleware.CORS())
	e.GET("/freee", func(c echo.Context) error {
		return c.JSON(http.StatusOK, "HELLO, WORLD!")
	})

	e.Logger.Fatal(e.Start(":8080"))
}
