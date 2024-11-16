package lua

// oslib_safe contains a subset of the lua OS library. For security reasons, we do not expose
// the entirety of lua OS library to custom actions, such as ones which can exit, read files, etc.
// Only the safe functions like os.time(), os.date() are exposed. Implementation was copied from
// github.com/yuin/gopher-lua.

import (
	"fmt"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func OpenSafeOs(l *lua.LState) int {
	tabmod := l.RegisterModule(lua.TabLibName, osFuncs)
	l.Push(tabmod)
	return 1
}

func SafeOsLoader(l *lua.LState) int {
	mod := l.SetFuncs(l.NewTable(), osFuncs)
	l.Push(mod)
	return 1
}

var osFuncs = map[string]lua.LGFunction{
	"time": osTime,
	"date": osDate,
}

func osTime(l *lua.LState) int {
	if l.GetTop() == 0 {
		l.Push(lua.LNumber(time.Now().Unix()))
	} else {
		tbl := l.CheckTable(1)
		sec := getIntField(tbl, "sec", 0)
		min := getIntField(tbl, "min", 0)
		hour := getIntField(tbl, "hour", 12)
		day := getIntField(tbl, "day", -1)
		month := getIntField(tbl, "month", -1)
		year := getIntField(tbl, "year", -1)
		isdst := getBoolField(tbl, "isdst", false)
		t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local)
		// TODO dst
		if false {
			print(isdst)
		}
		l.Push(lua.LNumber(t.Unix()))
	}
	return 1
}

func getIntField(tb *lua.LTable, key string, v int) int {
	ret := tb.RawGetString(key)
	if ln, ok := ret.(lua.LNumber); ok {
		return int(ln)
	}
	return v
}

func getBoolField(tb *lua.LTable, key string, v bool) bool {
	ret := tb.RawGetString(key)
	if lb, ok := ret.(lua.LBool); ok {
		return bool(lb)
	}
	return v
}

func osDate(l *lua.LState) int {
	t := time.Now()
	cfmt := "%c"
	if l.GetTop() >= 1 {
		cfmt = l.CheckString(1)
		if strings.HasPrefix(cfmt, "!") {
			t = time.Now().UTC()
			cfmt = strings.TrimLeft(cfmt, "!")
		}
		if l.GetTop() >= 2 {
			t = time.Unix(l.CheckInt64(2), 0)
		}
		if strings.HasPrefix(cfmt, "*t") {
			ret := l.NewTable()
			ret.RawSetString("year", lua.LNumber(t.Year()))
			ret.RawSetString("month", lua.LNumber(t.Month()))
			ret.RawSetString("day", lua.LNumber(t.Day()))
			ret.RawSetString("hour", lua.LNumber(t.Hour()))
			ret.RawSetString("min", lua.LNumber(t.Minute()))
			ret.RawSetString("sec", lua.LNumber(t.Second()))
			ret.RawSetString("wday", lua.LNumber(t.Weekday()+1))
			// TODO yday & dst
			ret.RawSetString("yday", lua.LNumber(0))
			ret.RawSetString("isdst", lua.LFalse)
			l.Push(ret)
			return 1
		}
	}
	l.Push(lua.LString(strftime(t, cfmt)))
	return 1
}

var cDateFlagToGo = map[byte]string{
	'a': "mon", 'A': "Monday", 'b': "Jan", 'B': "January", 'c': "02 Jan 06 15:04 MST", 'd': "02",
	'F': "2006-01-02", 'H': "15", 'I': "03", 'm': "01", 'M': "04", 'p': "PM", 'P': "pm", 'S': "05",
	'x': "15/04/05", 'X': "15:04:05", 'y': "06", 'Y': "2006", 'z': "-0700", 'Z': "MST",
}

func strftime(t time.Time, cfmt string) string {
	sc := newFlagScanner('%', "", "", cfmt)
	for c, eos := sc.Next(); !eos; c, eos = sc.Next() {
		if !sc.ChangeFlag {
			if sc.HasFlag {
				if v, ok := cDateFlagToGo[c]; ok {
					sc.AppendString(t.Format(v))
				} else {
					switch c {
					case 'w':
						sc.AppendString(fmt.Sprint(int(t.Weekday())))
					default:
						sc.AppendChar('%')
						sc.AppendChar(c)
					}
				}
				sc.HasFlag = false
			} else {
				sc.AppendChar(c)
			}
		}
	}

	return sc.String()
}

type flagScanner struct {
	flag       byte
	start      string
	end        string
	buf        []byte
	str        string
	Length     int
	Pos        int
	HasFlag    bool
	ChangeFlag bool
}

func newFlagScanner(flag byte, start, end, str string) *flagScanner {
	return &flagScanner{flag, start, end, make([]byte, 0, len(str)), str, len(str), 0, false, false}
}

func (fs *flagScanner) AppendString(str string) { fs.buf = append(fs.buf, str...) }

func (fs *flagScanner) AppendChar(ch byte) { fs.buf = append(fs.buf, ch) }

func (fs *flagScanner) String() string { return string(fs.buf) }

func (fs *flagScanner) Next() (byte, bool) {
	c := byte('\000')
	fs.ChangeFlag = false
	if fs.Pos == fs.Length {
		if fs.HasFlag {
			fs.AppendString(fs.end)
		}
		return c, true
	} else {
		c = fs.str[fs.Pos]
		if c == fs.flag {
			if fs.Pos < (fs.Length-1) && fs.str[fs.Pos+1] == fs.flag {
				fs.HasFlag = false
				fs.AppendChar(fs.flag)
				fs.Pos += 2
				return fs.Next()
			} else if fs.Pos != fs.Length-1 {
				if fs.HasFlag {
					fs.AppendString(fs.end)
				}
				fs.AppendString(fs.start)
				fs.ChangeFlag = true
				fs.HasFlag = true
			}
		}
	}
	fs.Pos++
	return c, false
}
