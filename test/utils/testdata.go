package utils

type TestData struct {
	Engines []*EngineConfig `yaml:"engines"`
}

type EngineConfig struct {
	Name      string     `yaml:"name"`
	SslConfig *SslConfig `yaml:"sslConfig"`
}

type SslConfig struct {
	Enabled    bool `yaml:"enabled"`
	SkipVerify bool `yaml:"skipVerify"`
}
