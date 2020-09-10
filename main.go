package main

import (
	"fmt"
	"github.com/bitfield/script"
	"github.com/shopspring/decimal"
	"github.com/urfave/cli/v2"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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

			startIndex := 1

			if c.Bool("dryRun") {
				return cutDryRun(parsedTimes, base, postfix, ext, startIndex)
			}

			return cutSchedule(parsedTimes, base, postfix, ext, startIndex, c.Bool("verbose"))
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
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
			return decTime{}, fmt.Errorf("invalid string to be parsed as int: %s, at i=%dm in %s, err = %w", s, i, integer, err)
		}
		if pi >= 60 {
			return decTime{}, fmt.Errorf("invalid string to be parsed as int: %s, at i=%dm in %s, err = %d >= 60", s, i, pi)
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

func cutDryRun(timePairs []decTimePair, base, postfix, ext string, startIndex int) error {
	in := base + ext

	for i, tp := range timePairs {
		command := constructCommand(tp, in, base, postfix, ext, i+startIndex, true)

		fmt.Println(command)
	}

	return nil
}

func cutSchedule(timePairs []decTimePair, base, postfix, ext string, startIndex int, verbose bool) error {
	in := base + ext

	for i, tp := range timePairs {
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