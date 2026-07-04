package infrastructure

// LoadStrategyFile reads KEY=value lines from a strategy file into a map (same format as resolvers).
func LoadStrategyFile(path string) (map[string]string, error) {
	return LoadResolverFile(path)
}
