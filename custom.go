package argp

import "strconv"

type Scanner interface {
	Scan([]string) (int, error)
}

// Count is a counting option, e.g. -vvv sets count to 3
type Count int

func (v *Count) Scan(s []string) (int, error) {
	if 0 < len(s) {
		if i, err := strconv.Atoi(s[0]); err == nil {
			*v = Count(i)
			return 0, nil
		}
	}
	*v++
	return 0, nil
}
