package discover

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Core struct {
	Name string
	Addr string
}

var (
	ErrNone   = errors.New("aucun Core annoncé sur le réseau")
	ErrChoice = errors.New("plusieurs Core, et ce terminal ne peut pas choisir")
)

func Choose(cores []Core, in io.Reader, out io.Writer, interactive bool) (Core, error) {
	switch len(cores) {
	case 0:
		return Core{}, ErrNone
	case 1:
		fmt.Fprintf(out, "Core %s  %s\n", cores[0].Name, cores[0].Addr)
		return cores[0], nil
	}
	if !interactive {
		fmt.Fprintln(out, "Plusieurs Core sont annoncés. Indiquez BRIGHTNET_BNCP_ADDR.")
		for _, core := range cores {
			fmt.Fprintf(out, "  %s  %s\n", core.Name, core.Addr)
		}
		return Core{}, ErrChoice
	}
	fmt.Fprintln(out, "Plusieurs Core sont annoncés.")
	for i, core := range cores {
		fmt.Fprintf(out, "  %d  %s  %s\n", i+1, core.Name, core.Addr)
	}
	reader := bufio.NewReader(in)
	for attempt := 0; attempt < 2; attempt++ {
		fmt.Fprint(out, "Numéro : ")
		line, err := reader.ReadString('\n')
		if err != nil && strings.TrimSpace(line) == "" {
			return Core{}, err
		}
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && n >= 1 && n <= len(cores) {
			fmt.Fprintf(out, "Core %s  %s\n", cores[n-1].Name, cores[n-1].Addr)
			return cores[n-1], nil
		}
		fmt.Fprintln(out, "Choix invalide.")
	}
	return Core{}, ErrChoice
}
