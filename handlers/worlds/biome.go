package worlds

type customBiome struct {
	name string
	data map[string]any
}

func (c *customBiome) EncodeBiome() int {
	return 0
}

func (c *customBiome) Temperature() float64 {
	return 0.5
}

func (c *customBiome) Rainfall() float64 {
	return 0.5
}

func (c *customBiome) String() string {
	return c.name
}
