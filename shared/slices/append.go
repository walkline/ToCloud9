package slices

func AppendBytes(b ...[]byte) []byte {
	l := 0
	for i := range b {
		l += len(b[i])
	}

	result := make([]byte, 0, l)
	for i := range b {
		result = append(result, b[i]...)
	}

	return result
}
