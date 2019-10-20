package monerotipbot

// Config holds all config parameters
type Config struct {
	entries map[string]string
}

// NewConfig initiates a new config
func NewConfig() *Config {
	return &Config{entries: make(map[string]string, 10)}
}

// GetConfig gets config parameters
func (c *Config) GetConfig() string {
	return c.entries["configpath"]
}

// SetConfig sets config parameters
func (c *Config) SetConfig(configpath string) {
	c.entries["configpath"] = configpath
}

// SetDebug sets debug parameter
func (c *Config) SetDebug(debug bool) {
	s := "false"
	if debug {
		s = "true"
	}
	c.entries["debug"] = s
}

// GetDebug gets debug parameter
func (c *Config) GetDebug() bool {
	if c.entries["debug"] == "true" {
		return true
	}
	return false
}
