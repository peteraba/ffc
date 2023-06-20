package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/bitfield/script"
	"github.com/shopspring/decimal"
	"github.com/urfave/cli/v2"
)

var timeArgRegex = regexp.MustCompile(`^[\d,.\-]+$`)

func main() {
	app := &cli.App{
		Name:  "ffcut",
		Usage: "cut video files via ffmpeg",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "seconds",
				Aliases: []string{"s"},
				Value:   false,
				Usage:   "use time provided as seconds (15123.3 -> 4 hours, 12 minutes, 3.3 seconds), default is to parse as clock (15123.3 -> 1 hours, 1 minute, 23.3 seconds)",
			},
			&cli.BoolFlag{
				Name:    "dryRun",
				Aliases: []string{"d"},
				Value:   false,
				Usage:   "create the command, but only print them, do not execute anything",
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Value:   false,
				Usage:   "print commands before executing them",
			},
			&cli.Float64Flag{
				Name:    "after-context",
				Aliases: []string{"a"},
				Value:   0.0,
				Usage:   "number of seconds to start cutting before the provided start point(s)",
			},
			&cli.Float64Flag{
				Name:    "before-context",
				Aliases: []string{"b"},
				Value:   0.0,
				Usage:   "number of seconds to end cutting after the provided end point(s)",
			},
			&cli.Float64Flag{
				Name:    "context",
				Aliases: []string{"c"},
				Value:   0.0,
				Usage:   "number of seconds to cut around the provided start and end point(s)",
			},
			&cli.StringFlag{
				Name:    "pick",
				Aliases: []string{"p"},
				Value:   "",
				Usage:   "comma separated list of part numbers to keep from args",
			},
		},
		Action: func(c *cli.Context) error {
			args := c.Args().Slice()

			base, postfix, ext, err := getFilenameParts(args)
			if err != nil {
				return err
			}

			rawTimes := collectTimes(args)

			parsedTimes, err := parseTimes(rawTimes, c.Bool("seconds"))
			if err != nil {
				return err
			}

			adjustedTimes, err := adjustTimes(parsedTimes, c.Float64("before-context"), c.Float64("after-context"), c.Float64("context"))
			if err != nil {
				return err
			}

			startIndex := 1
			picks, err := getPicks(c.String("pick"))
			if err != nil {
				return err
			}

			if c.Bool("dryRun") {
				return cutDryRun(adjustedTimes, base, postfix, ext, startIndex, picks)
			}

			return cutSchedule(adjustedTimes, base, postfix, ext, startIndex, picks, c.Bool("verbose"))
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func getPicks(arg string) (map[int]struct{}, error) {
	if len(arg) == 0 {
		return nil, nil
	}

	args := strings.Split(arg, ",")
	picks := make(map[int]struct{}, len(args))
	for _, a := range args {
		i, err := strconv.Atoi(a)
		if err != nil {
			return nil, err
		}

		picks[i-1] = struct{}{}
	}

	return picks, nil
}

func getFilenameParts(args []string) (string, string, string, error) {
	var (
		base, ext string
		postfix   = "ffc"
	)

	for _, arg := range args {
		if timeArgRegex.MatchString(arg) {
			continue
		}

		if base != "" {
			postfix = arg
			break
		}

		fi, err := os.Stat(arg)
		if err != nil {
			return "", "", "", err
		}

		if fi.IsDir() {
			return "", "", "", fmt.Errorf("provided path is a directory: %s", fi.Name())
		}

		ext = filepath.Ext(fi.Name())
		fn := arg
		if len(fn) < len(ext) {
			return "", "", "", fmt.Errorf("provided path is a weird, extension is not shorter than name: %s", fi.Name())
		}
		base = fn[:len(fn)-len(ext)]
	}

	if base == "" {
		return "", "", "", fmt.Errorf("no filename provided")
	}

	return base, postfix, ext, nil
}

type stringPair [2]string

func collectTimes(args []string) []stringPair {
	var res []stringPair

	for _, arg := range args {
		if !timeArgRegex.MatchString(arg) {
			continue
		}

		blocks := strings.Split(arg, ",")
		if len(blocks) < 1 {
			continue
		}

		for _, block := range blocks {
			res = append(res, parseBlock(block)...)
		}
	}

	return res
}

func parseBlock(block string) []stringPair {
	var res []stringPair

	if len(block) == 0 {
		return res
	}

	if block[0] == '-' {
		block = "0" + block
	}
	if block[len(block)-1] == '-' {
		block += "240000"
	}

	times := strings.Split(block, "-")
	if len(times) < 2 {
		return res
	}

	for i := 0; i < len(times)-1; i++ {
		res = append(res, [2]string{times[i], times[i+1]})
	}

	return res
}

type decTime struct {
	decimal.Decimal
}

func (dt decTime) String() string {
	base60 := intToTimeString(dt.IntPart())

	if dt.Exponent() <= 0 {
		return base60
	}

	return fmt.Sprint(base60, ".", dt.Exponent())
}

func intToTimeString(in int64) string {
	var (
		reverse, out []string
		n            int64
	)

	for {
		n = in % 60
		in /= 60

		if n > 9 {
			reverse = append(reverse, fmt.Sprint(n))
		} else {
			reverse = append(reverse, fmt.Sprint("0", n))
		}

		if in == 0 {
			break
		}
	}

	for i := len(reverse) - 1; i >= 0; i-- {
		out = append(out, fmt.Sprint(reverse[i]))
	}

	joined := strings.Join(out, ":")

	if len(joined) > 0 && joined[0] == '0' {
		return joined[1:]
	}

	return joined
}

type decTimePair [2]decTime

func (dtp decTimePair) isValid() bool {
	if dtp[0].Decimal.GreaterThan(dtp[1].Decimal) {
		return false
	}

	return true
}

func (dtp decTimePair) String() string {
	return dtp.From() + "+" + dtp.To()
}

func (dtp decTimePair) From() string {
	return dtp[0].String()
}

func (dtp decTimePair) To() string {
	return dtp[1].Decimal.Sub(dtp[0].Decimal).String()
}

func parseTimes(timePairs []stringPair, parseAsSeconds bool) ([]decTimePair, error) {
	var (
		a, b decTime
		p    decTimePair
		err  error
		res  []decTimePair
	)

	if parseAsSeconds {
		for _, tp := range timePairs {
			a, err = asSeconds(tp[0])
			if err != nil {
				return nil, err
			}

			b, err = asSeconds(tp[1])
			if err != nil {
				return nil, err
			}

			p = decTimePair{a, b}

			if !p.isValid() {
				return nil, fmt.Errorf("invalid decTimePair: %v (%s)", p, tp)
			}

			res = append(res, p)
		}

		return res, nil
	}

	for _, tp := range timePairs {
		a, err = asClock(tp[0])
		if err != nil {
			return nil, err
		}

		b, err = asClock(tp[1])
		if err != nil {
			return nil, err
		}

		p = decTimePair{a, b}

		if !p.isValid() {
			return nil, fmt.Errorf("invalid decTimePair: %v (%s)", p, tp)
		}

		res = append(res, p)
	}

	return res, nil
}

func asSeconds(in string) (decTime, error) {
	d, err := decimal.NewFromString(in)
	if err != nil {
		return decTime{}, err
	}

	return decTime{d}, nil
}

func asClock(in string) (decTime, error) {
	integer, exp, err := numSplit(in)
	if err != nil {
		return decTime{}, err
	}

	if len(integer)%2 == 1 {
		integer = "0" + integer
	}

	var (
		value      int
		pi         int64
		multiplier = 1
		s          string
		m          = len(integer)/2 - 1
	)
	for i := m; i >= 0; i-- {
		s = integer[i*2 : i*2+2]
		pi, err = strconv.ParseInt(s, 10, 32)
		if err != nil {
			return decTime{}, fmt.Errorf("invalid string to be parsed as int: %s, at i=%d in %s, err = %w", s, i, integer, err)
		}
		if pi >= 60 {
			return decTime{}, fmt.Errorf("pi is at least 60: %d", pi)
		}
		value += multiplier * int(pi)
		multiplier *= 60
	}

	in = fmt.Sprint(value)
	if exp > 0 {
		in = fmt.Sprint(value, ".", exp)
	}

	d, err := decimal.NewFromString(in)
	if err != nil {
		return decTime{}, err
	}

	return decTime{d}, nil
}

func numSplit(in string) (string, int32, error) {
	numParts := strings.Split(in, ".")

	if len(numParts) < 1 || len(numParts) > 2 {
		return "", 0, fmt.Errorf("invalid string to be parsed as clock: %s", in)
	}

	integer, fraction := numParts[0], ""
	if len(numParts) > 1 {
		fraction = numParts[1]
	}

	if fraction == "" {
		return integer, 0, nil
	}

	pf, err := strconv.ParseInt(fraction, 10, 32)
	if err != nil {
		return "", 0, fmt.Errorf("invalid string to be parsed as int: %s", fraction)
	}

	return integer, int32(pf), nil
}

const alpha = 0.0001

func adjustTimes(in []decTimePair, b, a, c float64) ([]decTimePair, error) {
	if len(in) == 0 || a < alpha && b < alpha && c < alpha {
		return in, nil
	}

	if a < alpha {
		a = c
	}
	if b < alpha {
		b = c
	}
	if a < 0.0 || b < 0.0 {
		return nil, fmt.Errorf("context must not be negative. a = %f, b = %f", a, b)
	}

	var out []decTimePair

	for _, tp := range in {
		s := tp[0].Add(decimal.NewFromFloat(-1 * b))
		e := tp[1].Add(decimal.NewFromFloat(a))
		if s.Cmp(decimal.Zero) < 0 {
			s = decimal.Zero
		}

		out = append(out, decTimePair{decTime{s}, decTime{e}})
	}

	return out, nil
}

func cutDryRun(timePairs []decTimePair, base, postfix, ext string, startIndex int, parts map[int]struct{}) error {
	in := base + ext

	for i, tp := range timePairs {
		if _, ok := parts[i]; !ok && len(parts) > 0 {
			continue
		}

		command := constructCommand(tp, in, base, postfix, ext, i+startIndex, true)

		fmt.Println(command)
	}

	return nil
}

func cutSchedule(timePairs []decTimePair, base, postfix, ext string, startIndex int, parts map[int]struct{}, verbose bool) error {
	in := base + ext

	for i, tp := range timePairs {
		if _, ok := parts[i]; !ok && len(parts) > 0 {
			continue
		}

		command := constructCommand(tp, in, base, postfix, ext, i+startIndex, verbose)

		if verbose {
			fmt.Println(command)
		}

		p := script.Exec(command)
		output, err := p.String()
		if err != nil {
			fmt.Println(err)
		} else if verbose {
			fmt.Println(output)
		}
	}

	return nil
}

func constructCommand(tp decTimePair, in, base, postfix, ext string, i int, verbose bool) string {
	out := constructName(base, postfix, ext, i, verbose)

	return fmt.Sprintf(`ffmpeg -ss %s -i "%s" -c copy -t %s "%s"`, tp.From(), in, tp.To(), out)
}

func constructName(base, postfix, ext string, i int, verbose bool) string {
	for {
		name := fmt.Sprintf("%s-%d%s%s", base, i, postfix, ext)
		if verbose {
			fmt.Println("Name constructed: ", name)
		}
		if _, err := os.Stat(name); os.IsNotExist(err) {
			return name
		}
		i += 1
	}
}
