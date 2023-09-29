package project

func difference(a, b []string) []string {
	return unique(append(a, b...))
}

func unique(slice []string) []string {
	encountered := map[string]int{}
	for _, v := range slice {
		encountered[v] = encountered[v] + 1
	}

	diff := make([]string, 0)
	for _, v := range slice {
		if encountered[v] == 1 {
			diff = append(diff, v)
		}
	}
	return diff
}
