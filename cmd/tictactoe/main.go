package main

import (
	"flag"
	"log"
	"os"
	"runtime/pprof"
	"runtime/trace"
	"time"

	"github.com/gorgonia/agogo"
	dual "github.com/gorgonia/agogo/dualnet"
	"github.com/gorgonia/agogo/game"
	"github.com/gorgonia/agogo/game/mnk"
	"github.com/gorgonia/agogo/mcts"

	"net/http"
	_ "net/http/pprof"
)

var (
	traceFlag  = flag.String("trace", "", "do a trace")
	cpuprofile = flag.String("cpuprofile", "", "cpuprofile")
)

func encodeBoard(a game.State) []float32 {
	board := agogo.EncodeTwoPlayerBoard(a.Board(), nil)
	for i := range board {
		if board[i] == 0 {
			board[i] = 0.001
		}
	}
	playerLayer := make([]float32, len(a.Board()))
	next := a.ToMove()
	if next == game.Player(game.Black) {
		for i := range playerLayer {
			playerLayer[i] = 1
		}
	} else if next == game.Player(game.White) {
		// vecf32.Scale(board, -1)
		for i := range playerLayer {
			playerLayer[i] = -1
		}
	}
	retVal := append(board, playerLayer...)
	return retVal
}

func main() {
	flag.Parse()
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	f, err := os.OpenFile("game.gif", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Unable to create gif file: %v", err)
	}
	defer f.Close()

	var Width = 6
	var WinSize = 4

	conf := agogo.Config{
		Name:            "Tic Tac Toe",
		NNConf:          dual.DefaultConf(Width, Width, 10),
		MCTSConf:        mcts.DefaultConfig(3),
		UpdateThreshold: 0.52,
	}
	conf.NNConf.BatchSize = 100
	conf.NNConf.Features = 2 // write a better encoding of the board, and increase features (and that allows you to increase K as well)
	conf.NNConf.K = 3
	conf.NNConf.SharedLayers = 3
	conf.MCTSConf = mcts.Config{
		PUCT:           1.0,
		M:              Width,
		N:              Width,
		Timeout:        100 * time.Millisecond,
		PassPreference: mcts.DontPreferPass,
		Budget:         1000,
		DumbPass:       true,
		RandomCount:    0,
	}

	outEnc := game.NewGifEncoder(300, 300)
	outEnc.Writer = f

	conf.Encoder = encodeBoard
	conf.OutputEncoder = outEnc

	if *traceFlag != "" {
		f, err := os.Create("trace.out")
		if err != nil {
			log.Fatalf("failed to create trace output file: %v", err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				log.Fatalf("failed to close trace file: %v", err)
			}
		}()

		if err := trace.Start(f); err != nil {
			log.Fatalf("failed to start trace: %v", err)
		}

		defer func() {
			<-time.After(10 * time.Second)
			trace.Stop()
		}()
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	g := mnk.New(Width, Width, WinSize)
	a := agogo.New(g, conf)
	a.Learn(5, 30, 200, 30) // 5 epochs, 50 episode, 100 NN iters, 100 games.
	outEnc.Flush()
	a.Save("example.model")
}
