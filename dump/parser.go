package dump

import (
	"bufio"
	"errors"
	"io"
	"regexp"
	"strconv"
)

var (
	ErrSkip = errors.New("Handler error, but skipped")
)

type ParseHandler interface {
	// Parse CHANGE MASTER TO MASTER_LOG_FILE=name, MASTER_LOG_POS=pos;
	BinLog(name string, pos uint64) error

	Data(schema string, table string, values []string) error
}

var binlogExp *regexp.Regexp
var useExp *regexp.Regexp
var valuesExp *regexp.Regexp

func init() {
	binlogExp = regexp.MustCompile("^CHANGE MASTER TO MASTER_LOG_FILE='(.+)', MASTER_LOG_POS=(\\d+);")
	useExp = regexp.MustCompile("^USE `(.+)`;")
	valuesExp = regexp.MustCompile("^INSERT INTO `(.+)` VALUES \\((.+)\\);")
}

// Parse the dump data with Dumper generate.
// It can not parse all the data formats with mysqldump outputs
func Parse(r io.Reader, h ParseHandler) error {
	rb := bufio.NewReaderSize(r, 1024*16)

	var db string
	var binlogParsed bool

	for {
		line, err := rb.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		} else if err == io.EOF {
			break
		}

		line = line[0 : len(line)-1]

		if !binlogParsed {
			if m := binlogExp.FindAllStringSubmatch(line, -1); len(m) == 1 {
				name := m[0][1]
				pos, err := strconv.ParseUint(m[0][2], 10, 64)
				if err != nil {
					return err
				}

				if err = h.BinLog(name, pos); err != nil && err != ErrSkip {
					return err
				}

				binlogParsed = true
			}
		}

		if m := useExp.FindAllStringSubmatch(line, -1); len(m) == 1 {
			db = m[0][1]
		}

		if m := valuesExp.FindAllStringSubmatch(line, -1); len(m) == 1 {
			table := m[0][1]

			values := parseValues(m[0][2])

			if err = h.Data(db, table, values); err != nil && err != ErrSkip {
				return err
			}
		}
	}

	return nil
}

func parseValues(str string) []string {
	// values are seperated by comma, but we can not split using comma directly
	// string is enclosed by single quote

	// a simple implementation, may be more robust later.

	values := make([]string, 0, 8)

	i := 0
	for i < len(str) {
		if str[i] != '\'' {
			// no string, read until comma
			j := i + 1
			for ; j < len(str) && str[j] != ','; j++ {
			}
			values = append(values, str[i:j])
			// skip ,
			i = j + 1
		} else {
			// read string until another single quote
			j := i + 1
			last := str[i]

			for ; !(str[j] == '\'' && last != '\\'); j++ {
				last = str[j]
			}
			values = append(values, str[i+1:j])
			// skip ' and ,
			i = j + 2
		}

		// need skip blank???
	}

	return values
}
