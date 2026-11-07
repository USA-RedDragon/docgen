// Package settings is the fixture of the docgen:configulator marker.
package settings

// Server holds the settings of the server.
type Server struct {
	Port int `name:"port" default:"8080" description:"port of the server"`
}

// Config holds the settings of the program.
type Config struct {
	Name   string `name:"name" default:"lamp" description:"name of the program"`
	Server Server `name:"server"`
}
