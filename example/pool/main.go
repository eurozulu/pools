package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/eurozulu/pools"
	"os"

	"strings"
)

func main() {
	ctx, cnl := context.WithCancel(context.Background())
	defer cnl()

	p := pools.NewPool[string](ctx, strings.Split("hello world, how you doing?\n", " ")...)
	for i := 0; i < 5; i++ {
		go drain(ctx, p, fmt.Sprintf("drain %d", i))
	}

	done := feed(ctx, p)
	fmt.Println("waiting")
	<-done
	fmt.Println("done")

}

func drain(ctx context.Context, p pools.Pool[string], id string) {
	for r := range p.Drain(ctx, 0) {
		fmt.Printf("%s: '%v'\n", id, r)
	}
}

func feed(ctx context.Context, p pools.Pool[string]) chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		chin := make(chan string)
		p.Feed(ctx, chin)
		scn := bufio.NewScanner(os.Stdin)
		for scn.Scan() {
			if scn.Text() == "exit" {
				return
			}
			select {
			case <-ctx.Done():
				return
			case chin <- scn.Text():
			}
		}
	}()
	return done
}
