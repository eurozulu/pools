package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/eurozulu/pools"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"
)

type PoolTest struct {
	Name string
}

func main() {
	ctx, cnl := context.WithCancel(context.Background())
	defer cnl()

	buf := bytes.NewBufferString("======Buffer log========\n")
	defer func(initLen int) {
		log.Printf("Closing pool with %d bytes in buffer\n", buf.Len()-initLen)
		log.Println(buf.String())
	}(buf.Len())

	log.Println("starting....")

	pt := NewPoolTest(strings.Split("hello world, how you doing?\n", " ")...)

	p := pools.NewPool[*PoolTest](ctx, pools.Policy{
		Count: 2,
	}, pt...)

	ctxx, cnlx := context.WithCancel(ctx)
	for i := 0; i < 500; i++ {
		name := fmt.Sprintf("buffer-read-%03d", i)
		go readPool(ctxx, p, name, buf)
	}
	defer cnlx()

	//go readPool(ctx, p, "console-read", os.Stdout)
	//go feedPool(ctx, p, "feed-haha", "ha", "ha")

	//go feedPool(ctx, p, "feed-mary", strings.Split("Mary had a little lamb", " ")...)

	//go feedPool(ctx, p, "console1")

	sig := make(chan os.Signal)
	signal.Notify(sig, os.Kill, os.Interrupt)
	select {
	case <-sig:
		return
	case <-time.After(time.Second):
		break
	}
	cnl()
	fmt.Printf("wait to close")
	p.WaitForClose()
	fmt.Printf("done")
}

func readPool[T any](ctx context.Context, pool pools.Pool[T], name string, out io.Writer) {
	fmt.Printf("%s: reading started\n", name)
	defer fmt.Printf("%s: reading closing\n", name)
	ch := pool.Read(ctx, -1)
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ch:
			fmt.Fprintf(out, "%s: %v\n", name, t)
		}
	}
}

func feedPool(ctx context.Context, p pools.Pool[*PoolTest], name string, items ...string) {
	fmt.Printf("%s: feeding started\n", name)
	defer fmt.Printf("%s: feeding closing\n", name)

	ch := make(chan *PoolTest)
	done := p.Feed(ctx, ch)
	var src io.Reader
	if len(items) == 0 {
		src = os.Stdin
	} else {
		src = bytes.NewBufferString(strings.Join(items, "\n"))
	}
	scn := bufio.NewScanner(src)
	for scn.Scan() {
		select {
		case <-ctx.Done():
			return
		case <-done:
			// Pool is no longer servicing request
			return
		case ch <- &PoolTest{Name: scn.Text()}:
		}
	}
}

func (pt PoolTest) String() string {
	return pt.Name
}

func NewPoolTest(s ...string) []*PoolTest {
	pt := make([]*PoolTest, len(s))
	for i, sz := range s {
		pt[i] = &PoolTest{Name: sz}
	}
	return pt
}
