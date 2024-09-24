package game

import (
	"log"
	"math/rand"
	"sync"
	"time"
	"tomb_mates/internal/protos"

	uuid "github.com/satori/go.uuid"
	"google.golang.org/protobuf/proto"
)

// Game represents game state
type Game struct {
	Mx              sync.Mutex
	Replica         bool
	Units           map[string]*protos.Unit
	UnitsCached     map[string]*protos.Unit
	UnitsSerialized *[]byte
	MyID            string
}

func New(isReplica bool, units map[string]*protos.Unit) *Game {
	world := &Game{
		Replica: isReplica,
		Units:   units,
	}

	return world
}

func (world *Game) AddPlayer() *protos.Unit {
	skins := []string{"big_demon", "big_zombie", "elf_f"}
	id := uuid.NewV4().String()
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	unit := &protos.Unit{
		Id: id,
		Position: &protos.Position{
			X: rnd.Float64()*300 + 10,
			Y: rnd.Float64()*220 + 10,
		},
		Frame:  int32(rnd.Intn(4)),
		Skin:   skins[rnd.Intn(len(skins))],
		Action: "idle",
		Velocity: &protos.Velocity{
			Direction: protos.Direction_left,
			Speed:     100,
		},
	}
	world.Units[id] = unit

	return unit
}

func (world *Game) HandleEvent(event *protos.Event) {
	world.Mx.Lock()
	defer world.Mx.Unlock()

	etype := event.GetType()

	switch etype {
	case protos.EventType_connect:
		data := event.GetConnect()
		world.Units[data.Unit.Id] = data.Unit

	case protos.EventType_init:
		data := event.GetInit()

		if world.Replica {
			world.MyID = data.PlayerId
		}

	case protos.EventType_exit:
		data := event.GetExit()
		delete(world.Units, data.PlayerId)

	case protos.EventType_move:
		data := event.GetMove()
		unit := world.Units[data.PlayerId]
		if unit == nil {
			return
		}
		unit.Action = UnitActionMove
		unit.Velocity.Direction = data.Direction

	case protos.EventType_idle:
		data := event.GetIdle()
		unit := world.Units[data.PlayerId]
		if unit == nil {
			return
		}
		unit.Action = UnitActionIdle

	case protos.EventType_state:
		data := event.GetState()
		units := data.GetUnits()
		if units != nil {
			world.Units = units
		}

	default:
		log.Println("UNKNOWN EVENT: ", event)
	}
}

func (world *Game) Run(tickRate time.Duration) {
	ticker := time.NewTicker(tickRate)
	lastEvolveTime := time.Now()

	for {
		select {
		case <-ticker.C:
			dt := time.Now().Sub(lastEvolveTime).Seconds()
			world.HandlePhysics(dt)
			lastEvolveTime = time.Now()
		}
	}
}

func (world *Game) HandlePhysics(dt float64) {
	for i := range world.Units {
		if world.Units[i].Action == UnitActionMove {
			switch world.Units[i].Velocity.Direction {
			case protos.Direction_left:
				world.Units[i].Position.X -= world.Units[i].Velocity.Speed * dt
				world.Units[i].Side = protos.Direction_left
			case protos.Direction_right:
				world.Units[i].Position.X += world.Units[i].Velocity.Speed * dt
				world.Units[i].Side = protos.Direction_right
			case protos.Direction_up:
				world.Units[i].Position.Y -= world.Units[i].Velocity.Speed * dt
			case protos.Direction_down:
				world.Units[i].Position.Y += world.Units[i].Velocity.Speed * dt
			default:
				log.Println("UNKNOWN DIRECTION: ", world.Units[i].Velocity.Direction)
			}
		}
	}

	// Cache units map
	world.Mx.Lock()
	world.UnitsCached = make(map[string]*protos.Unit, len(world.Units))
	for key, value := range world.Units {
		v := *value
		world.UnitsCached[key] = &v
	}
	world.Mx.Unlock()

	stateEvent := &protos.Event{
		Type: protos.EventType_state,
		Data: &protos.Event_State{
			State: &protos.GameState{
				Units: world.UnitsCached,
			},
		},
	}
	s, err := proto.Marshal(stateEvent)
	if err != nil {
		panic(err)
	}

	world.UnitsSerialized = &s
}

const UnitActionMove = "run"
const UnitActionIdle = "idle"