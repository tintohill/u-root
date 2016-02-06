// Copyright 2016 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	allProc = "^[0-9]+$"
	proc    = "/proc"
	USER_HZ = 100
)

// Portable way to implement ps cross-plataform
// Like the os.File
type Process struct {
	process
}

// table content of stat file defined by:
// https://www.kernel.org/doc/Documentation/filesystems/proc.txt (2009)
// Section (ctrl + f) : Table 1-4: Contents of the stat files (as of 2.6.30-rc7)
type process struct {
	Pid         string // process id
	Cmd         string // filename of the executable
	State       string // state (R is running, S is sleeping, D is sleeping in an uninterruptible wait, Z is zombie, T is traced or stopped)
	Ppid        string // process id of the parent process
	Pgrp        string // pgrp of the process
	Sid         string // session id
	TTYNr       string // tty the process uses
	TTYPgrp     string // pgrp of the tty
	Flags       string // task flags
	MinFlt      string // number of minor faults
	CminFlt     string // number of minor faults with child's
	MajFlt      string // number of major faults
	CmajFlt     string // number of major faults with child's
	Utime       string // user mode jiffies
	Stime       string // kernel mode jiffies
	Cutime      string // user mode jiffies with child's
	Cstime      string // kernel mode jiffies with child's
	Priority    string // priority level
	Nice        string // nice level
	NumThreads  string // number of threads
	ItRealValue string // (obsolete, always 0)
	StartTime   string // time the process started after system boot
	Vsize       string // virtual memory size
	Rss         string // resident set memory size
	Rsslim      string // current limit in bytes on the rss
	StartCode   string // address above which program text can run
	EndCode     string // address below which program text can run
	StartStack  string // address of the start of the main process stack
	Esp         string // current value of ESP
	Eip         string // current value of EIP
	Pending     string // bitmap of pending signals
	Blocked     string // bitmap of blocked signals
	Sigign      string // bitmap of ignored signals
	Sigcatch    string // bitmap of caught signals
	Wchan       string // place holder, used to be the wchan address, use /proc/PID/wchan
	Zero1       string // ignored
	Zero2       string // ignored
	ExitSignal  string // signal to send to parent thread on exit
	TaskCpu     string // which CPU the task is scheduled on
	RtPriority  string // realtime priority
	Policy      string // scheduling policy (man sched_setscheduler)
	BlkioTicks  string // time spent waiting for block IO
	Gtime       string // guest time of the task in jiffies
	Cgtime      string // guest time of the task children in jiffies
	StartData   string // address above which program data+bss is placed
	EndData     string // address below which program data+bss is placed
	StartBrk    string // address above which program heap can be expanded with brk()
	ArgStart    string // address above which program command line is placed
	ArgEnd      string // address below which program command line is placed
	EnvStart    string // address above which program environment is placed
	EnvEnd      string // address below which program environment is placed
	ExitCode    string // the thread's exit_code in the form reported by the waitpid system call (end of stat)
	Ctty        string // extra member (don't parsed from stat)
	Time        string // extra member (don't parsed from stat)
}

// Parse all content of stat to a Process Struct
// by gived the pid (linux)
func (p *process) readStat(pid string) error {
	b, err := ioutil.ReadFile(path.Join(proc, pid, "stat"))

	if err != nil {
		return err
	}

	fields := strings.Split(string(b), " ")

	// set struct fields from stat file data
	v := reflect.ValueOf(p).Elem()
	for i := 0; i < len(fields); i++ {
		fieldVal := v.Field(i)
		fieldVal.Set(reflect.ValueOf(fields[i]))
	}

	p.Time = p.getTime()
	p.Ctty = p.getCtty()
	cmd := p.Cmd
	p.Cmd = cmd[1 : len(cmd)-1]
	if flags.x {
		cmdline, err := p.longCmdLine()
		if err != nil {
			return err
		}
		if cmdline != "" {
			p.Cmd = cmdline
		}
	}

	return nil
}

// Fetch data from Operating System about process
// on Linux read data from stat
func (p *Process) Parse(pid string) error {
	if err := p.process.readStat(pid); err != nil {
		return err
	}

	return nil

}

// ctty returns the ctty or "?" if none can be found.
// TODO: an right way to get ctty by p.TTYNr and p.TTYPgrp
func (p process) getCtty() string {
	if tty, err := os.Readlink(path.Join(proc, p.Pid, "fd/0")); err != nil {
		return "?"
	} else if p.TTYPgrp != "-1" {
		if len(tty) > 5 && tty[:5] == "/dev/" {
			tty = tty[5:]
		}
		return tty
	}
	return "?"
}

// Get a named field of stat type
// e.g.: p.getField("Pid") => '1'
func (p *process) getField(field string) string {
	v := reflect.ValueOf(p).Elem()
	return fmt.Sprintf("%v", v.FieldByName(field))
}

// Search for attributes about the process
//
func (p Process) Search(field string) string {
	return p.process.getField(field)
}

// read UID of process based on or
func (p process) getUid() (int, error) {
	b, err := ioutil.ReadFile(path.Join(proc, p.Pid, "status"))

	var uid int
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Uid") {
			fields := strings.Split(line, "\t")
			uid, err = strconv.Atoi(fields[1])
			break
		}
	}

	return uid, err

}

func (p Process) GetUid() (int, error) {
	return p.process.getUid()
}

// change p.Cmd to long command line with args
func (p process) longCmdLine() (string, error) {
	b, err := ioutil.ReadFile(path.Join(proc, p.Pid, "cmdline"))

	if err != nil {
		return "", err
	}

	return string(b), nil
}

// Get total time stat formated hh:mm:ss
func (p process) getTime() string {
	utime, _ := strconv.Atoi(p.Utime)
	stime, _ := strconv.Atoi(p.Stime)
	jiffies := utime + stime

	tsecs := jiffies / USER_HZ
	secs := int(tsecs % 60)
	mins := int((tsecs / 60) % 60)
	hrs := tsecs / 3600

	return fmt.Sprintf("%02d:%02d:%02d", hrs, mins, secs)
}

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func terminalSize() (int, int) {
	ws := &winsize{}
	retCode, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)))

	if int(retCode) == -1 {
		panic(errno)
	}
	return int(ws.Row), int(ws.Col)
}

// Walk from the proc files
// and parsing them
func (pT *ProcessTable) LoadTable() error {
	pf := regexp.MustCompile(allProc)
	err := filepath.Walk(proc, func(name string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Printf("%v: %v\n", name, err)
			return err
		}
		if name == proc {
			return nil
		}

		if pf.Match([]byte(fi.Name())) {
			p := &Process{}
			if err := p.Parse(fi.Name()); err != nil {
				log.Print(err)
				return err
			}
			pT.table = append(pT.table, *p)
		}

		return filepath.SkipDir
	})

	// set terminal max on load table
	_, columnSize := terminalSize()
	pT.maxwidth = columnSize

	if err.Error() == "skip this directory" {
		return nil
	}

	return err
}