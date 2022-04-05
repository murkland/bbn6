package match

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/murkland/tango/input"
	"github.com/murkland/tango/mgba"
	"github.com/murkland/tango/replay"
)

type Battle struct {
	number int
	isP2   bool

	rw *replay.Writer

	iq *input.Queue

	localPendingTurnWaitTicksLeft int
	localPendingTurn              []byte

	isAcceptingInput bool

	lastCommittedRemoteInput input.Input

	committedState *mgba.State
	committedTick  int

	dirtyTick int
}

func (m *Match) NewBattle(core *mgba.Core) error {
	m.battleMu.Lock()
	defer m.battleMu.Unlock()

	if m.battle != nil {
		return errors.New("battle already started")
	}

	b := &Battle{
		number: m.battleNumber,
		isP2:   !m.wonLastBattle,

		lastCommittedRemoteInput: input.Input{Joyflags: 0xfc00},
	}

	b.iq = input.NewQueue(60, m.conf.Netplay.InputDelay, b.LocalPlayerIndex())

	fn := filepath.Join("replays", fmt.Sprintf("%s_p%d.tangoreplay", time.Now().Format("20060102030405"), b.LocalPlayerIndex()+1))
	log.Printf("writing replay: %s", fn)

	il, err := replay.NewWriter(fn, core)
	if err != nil {
		return err
	}
	b.rw = il
	m.battle = b
	log.Printf("battle %d started, won last battle (is p1) = %t", m.battleNumber, m.wonLastBattle)
	return nil
}

func (b *Battle) LocalPlayerIndex() int {
	if b.isP2 {
		return 1
	}
	return 0
}

func (b *Battle) RemotePlayerIndex() int {
	return 1 - b.LocalPlayerIndex()
}

func (b *Battle) QueueLength(playerIndex int) int {
	return b.iq.QueueLength(playerIndex)
}

func (b *Battle) PreIncrementDirtyTick() int {
	b.dirtyTick++
	return b.dirtyTick
}

func (b *Battle) DirtyTick() int {
	return b.dirtyTick
}

func (b *Battle) Close() error {
	if err := b.rw.Close(); err != nil {
		return err
	}
	return nil
}

func (b *Battle) SetCommittedTickAndState(tick int, state *mgba.State) {
	b.committedTick = tick
	b.committedState = state
}

func (b *Battle) CommittedTickAndState() (int, *mgba.State) {
	return b.committedTick, b.committedState
}

func (b *Battle) ConsumeInputs() ([][2]input.Input, []input.Input) {
	inputPairs := b.iq.Consume()
	if len(inputPairs) > 0 {
		b.lastCommittedRemoteInput = inputPairs[len(inputPairs)-1][1-b.LocalPlayerIndex()]
	}
	left := b.iq.PeekLocal()
	return inputPairs, left
}

func (b *Battle) AddInput(ctx context.Context, playerIndex int, input input.Input) error {
	return b.iq.AddInput(ctx, playerIndex, input)
}

func (b *Battle) AddLocalPendingTurn(turn []byte) {
	b.localPendingTurn = turn
	b.localPendingTurnWaitTicksLeft = 64
}

func (b *Battle) ConsumeLocalPendingTurn() []byte {
	var turn []byte
	if b.localPendingTurnWaitTicksLeft > 0 {
		b.localPendingTurnWaitTicksLeft--
		if b.localPendingTurnWaitTicksLeft == 0 {
			turn = b.localPendingTurn
			b.localPendingTurn = nil
		}
	}
	return turn
}

func (b *Battle) LastCommittedRemoteInput() input.Input {
	return b.lastCommittedRemoteInput
}

func (b *Battle) ReplayWriter() *replay.Writer {
	return b.rw
}

func (b *Battle) IsP2() bool {
	return b.isP2
}

func (b *Battle) LocalDelay() int {
	return b.iq.LocalDelay()
}

func (b *Battle) StartAcceptingInput() {
	b.isAcceptingInput = true
}

func (b *Battle) IsAcceptingInput() bool {
	return b.isAcceptingInput
}
