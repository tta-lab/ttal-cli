package format

// Plural returns singular when n == 1, otherwise returns the plural form.
func Plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
