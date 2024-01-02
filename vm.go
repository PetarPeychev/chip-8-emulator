package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"time"
)

// Register Names
const (
	V0 = iota
	V1
	V2
	V3
	V4
	V5
	V6
	V7
	V8
	V9
	VA
	VB
	VC
	VD
	VE
	VF
)

const FONTSET_START_ADDRESS = 0x50
const DISPLAY_WIDTH = 64
const DISPLAY_HEIGHT = 32

var FONTSET = [80]byte{
	0xF0, 0x90, 0x90, 0x90, 0xF0, // 0
	0x20, 0x60, 0x20, 0x20, 0x70, // 1
	0xF0, 0x10, 0xF0, 0x80, 0xF0, // 2
	0xF0, 0x10, 0xF0, 0x10, 0xF0, // 3
	0x90, 0x90, 0xF0, 0x10, 0x10, // 4
	0xF0, 0x80, 0xF0, 0x10, 0xF0, // 5
	0xF0, 0x80, 0xF0, 0x90, 0xF0, // 6
	0xF0, 0x10, 0x20, 0x40, 0x40, // 7
	0xF0, 0x90, 0xF0, 0x90, 0xF0, // 8
	0xF0, 0x90, 0xF0, 0x10, 0xF0, // 9
	0xF0, 0x90, 0xF0, 0x90, 0x90, // A
	0xE0, 0x90, 0xE0, 0x90, 0xE0, // B
	0xF0, 0x80, 0x80, 0x80, 0xF0, // C
	0xE0, 0x90, 0x90, 0x90, 0xE0, // D
	0xF0, 0x80, 0xF0, 0x80, 0xF0, // E
	0xF0, 0x80, 0xF0, 0x80, 0x80, // F
}

type VM struct {
	memory         [4096]byte
	display        [64][32]bool
	displayUpdated bool
	pc             uint16
	i              uint16
	stack          [16]uint16
	sp             byte
	delay          byte
	sound          byte
	registers      [16]byte
	keys           [16]bool
}

func NewVM() *VM {
	vm := new(VM)
	vm.pc = 0x200
	for i := 0; i < len(FONTSET); i++ {
		vm.memory[FONTSET_START_ADDRESS+i] = FONTSET[i]
	}
	rand.Seed(time.Now().UnixNano())
	return vm
}

func (vm *VM) LoadProgram(ram []byte) {
	for i := 0; i < len(ram); i++ {
		// 0x000 - 0x1FF was historically reserved for the interpreter
		vm.memory[i+0x200] = ram[i]
	}
}

func (vm *VM) LoadProgramFromFile(filename string) error {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	vm.LoadProgram(contents)
	return nil
}

func (vm *VM) fetch() uint16 {
	instruction := uint16(vm.memory[vm.pc])<<8 | uint16(vm.memory[vm.pc+1])
	vm.pc += 2
	return instruction
}

func (vm *VM) execute(instruction uint16) {
	code := instruction & 0xF000     // instruction code
	x := (instruction & 0x0F00) >> 8 // first register index
	y := (instruction & 0x00F0) >> 4 // second register index
	n := instruction & 0x000F        // 4-bit immediate value
	nn := instruction & 0x00FF       // 8-bit immediate value
	nnn := instruction & 0x0FFF      // 12-bit immediate value

	switch code {
	// execute built-in function
	case 0x0000:
		switch instruction {
		// clear display
		case 0x00E0:
			for i := 0; i < DISPLAY_WIDTH; i++ {
				for j := 0; j < DISPLAY_HEIGHT; j++ {
					vm.display[i][j] = false
				}
			}
			vm.displayUpdated = true
		// return from subroutine
		case 0x00EE:
			vm.sp--
			vm.pc = vm.stack[vm.sp]
		}

	// jump to address
	case 0x1000:
		vm.pc = nnn

	// call subroutine
	case 0x2000:
		vm.stack[vm.sp] = vm.pc
		vm.sp++
		vm.pc = nnn

	// skip next instruction if register equal to value
	case 0x3000:
		if vm.registers[x] == byte(nn) {
			vm.pc += 2
		}

	// skip next instruction if register not equal to value
	case 0x4000:
		if vm.registers[x] != byte(nn) {
			vm.pc += 2
		}

	// skip next instruction if register equal to register
	case 0x5000:
		if vm.registers[x] == vm.registers[y] {
			vm.pc += 2
		}

	// set register to value
	case 0x6000:
		vm.registers[x] = byte(nn)

	// add value to register
	case 0x7000:
		vm.registers[x] += byte(nn)

	// logic and arithmetic instructions
	case 0x8000:
		switch instruction & 0x000F {
		// set register to register
		case 0x0000:
			vm.registers[x] = vm.registers[y]

		// bitwise or
		case 0x0001:
			vm.registers[x] |= vm.registers[y]

		// bitwise and
		case 0x0002:
			vm.registers[x] &= vm.registers[y]

		// bitwise xor
		case 0x0003:
			vm.registers[x] ^= vm.registers[y]

		// add two registers, set VF to 1 if carry
		case 0x0004:
			vm.registers[x] += vm.registers[y]
			if vm.registers[y] > 0xFF-vm.registers[x] {
				vm.registers[VF] = 1
			} else {
				vm.registers[VF] = 0
			}

		// subtract two registers, set VF to 0 if borrow
		case 0x0005:
			fallthrough
		case 0x0007:
			if vm.registers[y] > vm.registers[x] {
				vm.registers[VF] = 0
			} else {
				vm.registers[VF] = 1
			}
			vm.registers[x] -= vm.registers[y]

		// shift right (not using Y register)
		case 0x0006:
			fallthrough
		case 0x000E:
			vm.registers[VF] = vm.registers[x] & 0x1
			vm.registers[x] >>= 1

		// skip next instruction if register not equal to register
		case 0x9000:
			if vm.registers[x] != vm.registers[y] {
				vm.pc += 2
			}
		}

	// set index register to value
	case 0xA000:
		vm.i = nnn

	// jump to address plus register
	case 0xB000:
		vm.pc = nnn + uint16(vm.registers[0])

	// set register to random value and mask with value
	case 0xC000:
		vm.registers[x] = byte(nn) & byte(rand.Intn(256))

	// draw sprite
	case 0xD000:
		xpos := uint16(vm.registers[x] % DISPLAY_WIDTH)
		ypos := uint16(vm.registers[y] % DISPLAY_HEIGHT)
		vm.registers[VF] = 0
		for row := uint16(0); row < n; row++ {
			if ypos+row >= DISPLAY_HEIGHT {
				break
			}
			spriteRow := vm.memory[vm.i+row]
			for col := uint16(0); col < 8; col++ {
				if xpos+col >= DISPLAY_WIDTH {
					break
				}
				spritePixel := spriteRow & (0x80 >> col)
				displayPixel := vm.display[xpos+col][ypos+row]
				if spritePixel == 1 {
					if displayPixel {
						vm.registers[VF] = 1
					}
					vm.display[xpos+col][ypos+row] = !displayPixel
				}
			}
		}
		vm.displayUpdated = true

	// input instructions
	case 0xE000:
		switch instruction & 0x00FF {
		// skip next instruction if key pressed
		case 0x009E:
			if vm.keys[vm.registers[x]] {
				vm.pc += 2
			}

		// skip next instruction if key not pressed
		case 0x00A1:
			if !vm.keys[vm.registers[x]] {
				vm.pc += 2
			}
		}

	// miscellaneous instructions
	case 0xF000:
		switch instruction & 0x00FF {
		// set delay timer to value
		case 0x0007:
			vm.registers[x] = vm.delay

		// wait for key press, store key value in register
		case 0x000A:
			keyPressed := false
			for i := 0; i < len(vm.keys); i++ {
				if vm.keys[i] {
					vm.registers[x] = byte(i)
					keyPressed = true
					break
				}
			}
			if !keyPressed {
				vm.pc -= 2
			}

		// set delay timer to register
		case 0x0015:
			vm.delay = vm.registers[x]

		// set sound timer to register
		case 0x0018:
			vm.sound = vm.registers[x]

		// add register to index register
		case 0x001E:
			vm.i += uint16(vm.registers[x])

		// set index register to location of sprite for digit
		case 0x0029:
			vm.i = FONTSET_START_ADDRESS + uint16(vm.registers[x])*5

		// store binary-coded decimal representation of register
		case 0x0033:
			vm.memory[vm.i] = vm.registers[x] / 100
			vm.memory[vm.i+1] = (vm.registers[x] / 10) % 10
			vm.memory[vm.i+2] = (vm.registers[x] % 100) % 10

		// store registers in memory starting at index register
		case 0x0055:
			for i := 0; i <= int(x); i++ {
				vm.memory[vm.i+uint16(i)] = vm.registers[i]
			}

		// read registers from memory starting at index register
		case 0x0065:
			for i := 0; i <= int(x); i++ {
				vm.registers[i] = vm.memory[vm.i+uint16(i)]
			}
		}
	}
}

func (vm *VM) render() {
	c := exec.Command("clear")
	c.Stdout = os.Stdout
	c.Run()
	for i := 0; i < DISPLAY_WIDTH; i++ {
		for j := 0; j < DISPLAY_HEIGHT; j++ {
			if vm.display[i][j] {
				print("█")
			} else {
				print(" ")
			}
		}
		println()
	}
}

func (vm *VM) Run() {
	for {
		time.Sleep(10 * time.Millisecond)
		instruction := vm.fetch()
		fmt.Printf("%04X: %04X\n", vm.pc, instruction)
		vm.execute(instruction)
		if vm.displayUpdated {
			// vm.render()
			vm.displayUpdated = false
		}
	}
}
