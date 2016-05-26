package reminders

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/itsabot/abot/shared/datatypes"
	"github.com/itsabot/abot/shared/helpers/timeparse"
	"github.com/itsabot/abot/shared/language"
	"github.com/itsabot/abot/shared/plugin"
)

var p *dt.Plugin

func init() {
	var err error
	p, err = plugin.New("github.com/itsabot/plugin_reminders")
	if err != nil {
		p.Log.Fatal(err)
	}
	plugin.SetKeywords(p,
		dt.KeywordHandler{
			Fn: kwSetReminder,
			Trigger: &dt.StructuredInput{
				Commands: []string{"remind"},
				Objects:  []string{"me"},
			},
		},
		/*dt.KeywordHandler{
			Fn: kwDeleteReminder,
			Trigger: &dt.StructuredInput{
				Commands: []string{"delete", "remove", "forget",
					"drop"},
				Objects: []string{"reminder"},
			},
		},*/
	)
	plugin.SetStates(p, [][]dt.State{[]dt.State{
		dt.State{
			OnEntry: func(in *dt.Msg) string {
				return "What would you like me to remind you to do?"
			},
			OnInput: func(in *dt.Msg) {
				var s string
				for _, w := range in.Tokens {
					switch strings.ToLower(w) {
					case "remind", "me", "to", "later":
						continue
					}
					s += w + " "
				}
				s = s[:len(s)-1]
				p.SetMemory(in, "reminderContent", s)
			},
			Complete: func(in *dt.Msg) (bool, string) {
				return p.HasMemory(in, "reminderContent"), ""
			},
			SkipIfComplete: true,
		},
		dt.State{
			OnEntry: func(in *dt.Msg) string {
				return "Ok. When should I remind you?"
			},
			OnInput: func(in *dt.Msg) {
				ts := timeparse.Parse(in.Sentence)
				if len(ts) == 0 {
					p.Log.Info("FOUND NO TIMES")
					return
				}
				p.SetMemory(in, "reminderTime", ts[0])
			},
			Complete: func(in *dt.Msg) (bool, string) {
				return p.HasMemory(in, "reminderTime"), ""
			},
			SkipIfComplete: true,
		},
		dt.State{
			OnEntry: func(in *dt.Msg) string {
				c := p.GetMemory(in, "reminderContent").String()
				mem := p.GetMemory(in, "reminderTime")
				var t time.Time
				if err := json.Unmarshal(mem.Val, &t); err != nil {
					p.Log.Info("failed to unmarshal time.", err)
					return ""
				}
				ts := t.Format("Monday at 3:04PM")
				return fmt.Sprintf("Ok. I'll remind you to %s %s.", c, ts)
			},
			OnInput: func(in *dt.Msg) {},
			Complete: func(in *dt.Msg) (bool, string) {
				return true, ""
			},
		},
	}})
	p.SM.SetOnReset(func(in *dt.Msg) {
		p.DeleteMemory(in, "reminderContent")
		p.DeleteMemory(in, "reminderTime")
	})
	if err = plugin.Register(p); err != nil {
		p.Log.Fatal(err)
	}
}

func kwSetReminder(in *dt.Msg) string {
	p.Log.Debug("parsing", in.Sentence)
	// Count and locate prepositions in the sentence
	var prepLocs []int
	for i, w := range in.Tokens {
		_, yes := language.Prepositions[w]
		if yes {
			prepLocs = append(prepLocs, i)
			p.Log.Debug("found preposition:", w)
		}
	}

	// If no prepositions were found, this is an invalid reminder request
	if len(prepLocs) == 0 {
		p.Log.Debug("found no prepositions. returning")
		return ""
	}

	// Check the first preposition. If the first preposition is "to", the
	// words that follow (until the final preposition in the sentence) are
	// the reminder content
	var final bool
	var timeStr, reminderStr string
	if in.Tokens[prepLocs[0]] == "to" {
		p.Log.Debug("found \"to\" preposition")
		if len(prepLocs) == 1 {
			// TODO remove final punctuation
			reminderStr = strings.Join(in.Tokens[prepLocs[0]+1:], " ")
			p.SetMemory(in, "reminderContent", reminderStr)
			final = true
		} else {
			reminderStr = strings.Join(in.Tokens[prepLocs[0]+1:prepLocs[len(prepLocs)-1]], " ")
			p.SetMemory(in, "reminderContent", reminderStr)
		}
	} else {
		// If it's not "to", the content until the next preposition
		// must be a time.
		p.Log.Debug("found a non-\"to\" preposition")
		if len(prepLocs) == 1 {
			timeStr = strings.Join(in.Tokens[prepLocs[0]+1:], " ")
			final = true
		} else {
			timeStr = strings.Join(in.Tokens[prepLocs[0]+1:], " ")
		}
	}

	// Parse the timestring into time.Time
	if len(timeStr) > 0 {
		p.Log.Debug("parsing a timestring")
		ts := timeparse.Parse(timeStr)
		if len(ts) == 0 && final {
			return ""
		}
		p.SetMemory(in, "reminderTime", ts[0])
	}

	// If we've extracted all we can out of the sentence at this point,
	// we've only extracted time OR content--not both. Therefore return as
	// invalid and the state machine will handle the rest.
	var ts []time.Time
	if final {
		// Parse all remaining content looking for times
		c := strings.Join(in.Tokens[prepLocs[0]+1:], " ")
		ts = timeparse.Parse(c)
		if len(ts) == 0 {
			return ""
		}
		if len(ts) > 0 {
			p.SetMemory(in, "reminderTime", ts[0])
		}
		p.Log.Debug("extracted all data from the sentence")
	}

	// There's remaining content in the sentence. Assume it's the opposite
	// type, i.e. if we have reminder content, assume the remainder is
	// time. If we have a time, assume it's reminder content.
	if len(reminderStr) > 0 {
		r := strings.Join(in.Tokens[prepLocs[len(prepLocs)-1]+1:], " ")
		p.Log.Debug("parsing the remaining timestring:", r)
		ts = timeparse.Parse(r)
		if len(ts) == 0 {
			return ""
		}
		p.SetMemory(in, "reminderTime", ts[0])
	} else {
		return ""
	}
	err := p.Schedule(in, "Hey! Remember to "+reminderStr, ts[0])
	if err != nil {
		p.Log.Info("failed to schedule reminder.", err)
	}
	timeStr = ts[0].Format("Monday at 3:04PM")
	return fmt.Sprintf("Ok. I'll remind you to %s %s.", reminderStr,
		timeStr)
}
