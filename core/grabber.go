package core

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	dateQueryJitterMaxMs = 40
	submitMinIntervalMs  = 1800
	submitBackoffMinMs   = 2500
	submitBackoffMaxMs   = 4200
)

type GrabResult struct {
	Success bool
	Message string
	Detail  *GrabSuccess
	Err     error
}

type Grabber struct {
	client       *HealthClient
	lastSubmitAt time.Time
}

func NewGrabber(client *HealthClient) *Grabber {
	return &Grabber{client: client}
}

func (g *Grabber) Run(ctx context.Context, input map[string]any, onLog func(level, message string)) GrabResult {
	if g == nil || g.client == nil {
		return GrabResult{Success: false, Message: "client not initialized", Err: errors.New("client not initialized")}
	}
	config, err := parseGrabConfig(input)
	if err != nil {
		emitLog(onLog, "error", err.Error())
		return GrabResult{Success: false, Message: err.Error(), Err: err}
	}

	if ctx == nil {
		ctx = context.Background()
	}

	emitLog(onLog, "info", "grab engine started")
	emitLog(onLog, "info", fmt.Sprintf(
		"grab config: dates=%s doctor_ids=%s time_types=%s preferred=%s",
		strings.Join(config.TargetDates, ","),
		strings.Join(config.DoctorIDs, ","),
		strings.Join(config.TimeTypes, ","),
		strings.Join(config.PreferredHours, ","),
	))
	isPrecise := len(config.DoctorIDs) > 0 || len(config.PreferredHours) > 0 || len(config.TimeTypes) > 0
	if isPrecise {
		emitLog(onLog, "info", "grab mode: precise")
	} else {
		emitLog(onLog, "info", "grab mode: fuzzy")
	}
	if len(config.TimeTypes) == 0 {
		emitLog(onLog, "info", "time_types 未设置，默认 am/pm")
	}
	if config.StartTime != "" {
		// Parse StartTime to absolute time (Today)
		now := time.Now()
		parsedStart, err := time.Parse("15:04:05", config.StartTime)
		if err != nil {
			emitLog(onLog, "error", fmt.Sprintf("invalid start_time format: %s", config.StartTime))
			return GrabResult{Success: false, Message: "invalid start_time", Err: err}
		}
		targetTime := time.Date(now.Year(), now.Month(), now.Day(), parsedStart.Hour(), parsedStart.Minute(), parsedStart.Second(), 0, now.Location())

		// Pre-grab test logic
		if config.PreGrabTestEnabled {
			offset := config.PreGrabTestOffsetSeconds
			if offset <= 0 {
				offset = 60
			}

			// Calculate absolute pre-test time
			preTestTime := targetTime.Add(-time.Duration(offset) * time.Second)

			emitLog(onLog, "info", fmt.Sprintf("waiting for pre-grab test at %s (-%ds)", preTestTime.Format("15:04:05"), offset))
			waitUntil(ctx, preTestTime, g.client, config.UseServerTime, onLog)

			if ctx.Err() == nil {
				emitLog(onLog, "info", "starting pre-grab test (query only)")
				ok, err := g.testGrabOnce(ctx, config, onLog)
				if err != nil {
					emitLog(onLog, "warn", fmt.Sprintf("pre-grab test failed: %v", err))
				} else if ok {
					emitLog(onLog, "success", "pre-grab test connection OK")
				} else {
					emitLog(onLog, "warn", "pre-grab test: no schedule found (connection OK)")
				}
			}
		}

		if ctx.Err() != nil {
			return GrabResult{Success: false, Message: "stopped", Err: ctx.Err()}
		}

		emitLog(onLog, "info", fmt.Sprintf("waiting for target time: %s", config.StartTime))
		waitUntil(ctx, targetTime, g.client, config.UseServerTime, onLog)
		if ctx.Err() != nil {
			return GrabResult{Success: false, Message: "stopped", Err: ctx.Err()}
		}
	}

	attempt := 0
	retryInterval := config.RetryInterval
	if retryInterval <= 0 {
		retryInterval = 0.5
	}

	for {
		if ctx.Err() != nil {
			return GrabResult{Success: false, Message: "stopped", Err: ctx.Err()}
		}
		attempt++
		emitLog(onLog, "info", fmt.Sprintf("attempt %d", attempt))

		success, fatalErr := g.tryGrabOnce(ctx, config, onLog)
		if fatalErr != nil {
			return GrabResult{Success: false, Message: fatalErr.Error(), Err: fatalErr}
		}
		if success != nil {
			emitLog(onLog, "success", "grab success")
			return GrabResult{Success: true, Message: "success", Detail: success}
		}

		if config.MaxRetries > 0 && attempt >= config.MaxRetries {
			emitLog(onLog, "warn", fmt.Sprintf("max retries reached (%d)", config.MaxRetries))
			return GrabResult{Success: false, Message: "max retries reached"}
		}

		if !sleepWithContext(ctx, time.Duration(retryInterval*1000)*time.Millisecond) {
			return GrabResult{Success: false, Message: "stopped", Err: ctx.Err()}
		}
	}
}

func (g *Grabber) testGrabOnce(ctx context.Context, config GrabConfig, onLog func(level, message string)) (bool, error) {
	if len(config.TargetDates) == 0 {
		return false, nil
	}
	// Only query the first date to test connection
	date := config.TargetDates[0]
	unitID := config.UnitID
	depID := config.DepID

	emitLog(onLog, "info", fmt.Sprintf("pre-grab test query: %s", date))
	docs, err := g.client.GetSchedule(unitID, depID, date)
	if err != nil {
		return false, err
	}
	return len(docs) > 0, nil
}

// calculatePreTestTime removed as logic moved to Run
// func calculatePreTestTime(targetTimeStr string, offsetSeconds int) (string, error) { ... }

func (g *Grabber) tryGrabOnce(ctx context.Context, config GrabConfig, onLog func(level, message string)) (*GrabSuccess, error) {
	unitID := config.UnitID
	depID := config.DepID
	memberID := config.MemberID
	doctorSet := toSet(config.DoctorIDs)
	timeSet := toSet(config.TimeTypes)
	if len(timeSet) == 0 {
		timeSet = toSet([]string{"am", "pm"})
	}

	if ctx == nil {
		ctx = context.Background()
	}

	targetDates := config.TargetDates
	if len(targetDates) == 0 {
		return nil, nil
	}

	for _, date := range targetDates {
		date := date
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if dateQueryJitterMaxMs > 0 {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			time.Sleep(time.Duration(r.Intn(dateQueryJitterMaxMs)) * time.Millisecond)
		}
		success, err := g.tryGrabDate(ctx, config, unitID, depID, memberID, date, doctorSet, timeSet, onLog)
		if err != nil {
			return nil, err
		}
		if success != nil {
			return success, nil
		}
	}
	return nil, nil
}

func (g *Grabber) tryGrabDate(
	ctx context.Context,
	config GrabConfig,
	unitID, depID, memberID, date string,
	doctorSet, timeSet map[string]struct{},
	onLog func(level, message string),
) (*GrabSuccess, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	emitLog(onLog, "info", fmt.Sprintf("schedule query: %s", date))
	docs, err := g.client.GetSchedule(unitID, depID, date)
	if err != nil {
		if errors.Is(err, ErrLoginRequired) {
			emitLog(onLog, "error", "登录已失效，请重新扫码")
			return nil, fmt.Errorf("%w: 登录已失效，请重新扫码", ErrLoginRequired)
		}
		emitLog(onLog, "error", fmt.Sprintf("schedule error: %v", err))
		return nil, nil
	}
	if len(docs) == 0 {
		emitLog(onLog, "warn", fmt.Sprintf("no schedule on %s", date))
		return nil, nil
	}

	matchedDoctor := 0
	checkedSlots := 0
	matchedSlots := 0
	filteredByTime := 0
	isPrecise := len(config.DoctorIDs) > 0 || len(config.PreferredHours) > 0 || len(config.TimeTypes) > 0

	emitLog(onLog, "info", fmt.Sprintf("schedule result: docs=%d", len(docs)))

	totalSlots := 0
	timeTypeCounts := map[string]int{}
	leftByType := map[string]int{}
	samples := make([]string, 0, 3)

	for _, doc := range docs {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		docID := toString(doc["doctor_id"])
		if len(doctorSet) > 0 && !containsSet(doctorSet, docID) {
			continue
		}
		matchedDoctor++
		schedules := asSlice(doc["schedules"])
		for _, rawSlot := range schedules {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			slot := asMap(rawSlot)
			if slot == nil {
				continue
			}
			timeType := toString(slot["time_type"])
			leftNum := toInt(slot["left_num"])
			totalSlots++
			timeTypeCounts[timeType]++
			if leftNum > 0 {
				leftByType[timeType]++
			}
			if len(samples) < 3 {
				samples = append(samples, fmt.Sprintf("%s/%s left=%d id=%s",
					timeType,
					toString(slot["time_type_desc"]),
					leftNum,
					toString(slot["schedule_id"]),
				))
			}
			if len(timeSet) > 0 && !containsSet(timeSet, timeType) {
				filteredByTime++
				continue
			}
			checkedSlots++
			if leftNum <= 0 {
				continue
			}
			matchedSlots++
			scheduleID := toString(slot["schedule_id"])
			if scheduleID == "" {
				continue
			}
			docName := toString(doc["doctor_name"])
			timeDesc := toString(slot["time_type_desc"])
			emitLog(onLog, "success", fmt.Sprintf("found slot: %s - %s (left %d)", docName, timeDesc, leftNum))

			detail, err := g.client.GetTicketDetail(unitID, depID, scheduleID, memberID)
			if err != nil || detail == nil {
				emitLog(onLog, "warn", "ticket detail unavailable")
				continue
			}
			times := detail.Times
			if len(times) == 0 {
				times = detail.TimeSlots
			}
			if len(times) == 0 {
				continue
			}
			if detail.SchData == "" || detail.DetlidRealtime == "" || detail.LevelCode == "" {
				emitLog(onLog, "warn", "ticket detail missing fields")
				continue
			}

			selected := pickTimeSlot(times, config.PreferredHours)
			emitLog(onLog, "info", fmt.Sprintf("selected time slot: %s", selected.Name))

			addressID, addressText := resolveAddress(config, detail, onLog)
			if addressID == "" || addressText == "" {
				emitLog(onLog, "error", "missing address info")
				continue
			}

			submitParams := map[string]any{
				"unit_id":         unitID,
				"dep_id":          depID,
				"schedule_id":     scheduleID,
				"time_type":       timeType,
				"doctor_id":       docID,
				"his_doc_id":      toString(doc["his_doc_id"]),
				"his_dep_id":      toString(doc["his_dep_id"]),
				"detlid":          selected.Value,
				"member_id":       memberID,
				"addressId":       addressID,
				"address":         addressText,
				"sch_data":        detail.SchData,
				"level_code":      detail.LevelCode,
				"detlid_realtime": detail.DetlidRealtime,
				"sch_date":        detail.SchDate,
				"hisMemId":        detail.HisMemID,
				"order_no":        detail.OrderNo,
				"disease_input":   detail.DiseaseInput,
				"disease_content": detail.DiseaseContent,
				"is_hot":          detail.IsHot,
			}
			result, err := g.submitOnce(ctx, submitParams, onLog)
			if err != nil {
				emitLog(onLog, "error", fmt.Sprintf("submit error: %v", err))
				continue
			}
			if result != nil && (result.Success || result.Status) {
				unitName := fallback(config.UnitName, config.UnitID)
				depName := fallback(config.DepName, config.DepID)
				memberName := fallback(config.MemberName, config.MemberID)
				success := &GrabSuccess{
					UnitName:   unitName,
					DepName:    depName,
					DoctorName: docName,
					Date:       date,
					TimeSlot:   selected.Name,
					MemberName: memberName,
					URL:        result.URL,
				}
				emitLog(onLog, "success", fmt.Sprintf("success: %s / %s / %s", unitName, depName, docName))
				return success, nil
			}

			msg := ""
			if result != nil {
				msg = result.Message
			}
			if msg == "" {
				msg = "submit failed"
			}
			if isTooFastMessage(msg) {
				proxyURL := ""
				if config.UseProxySubmit && g.client != nil {
					rotated, err := g.client.RotateProxy("", "")
					if err != nil {
						emitLog(onLog, "warn", fmt.Sprintf("proxy rotate failed: %v", err))
					} else {
						proxyURL = rotated
						emitLog(onLog, "info", fmt.Sprintf("proxy switched: %s", proxyURL))
					}
				} else if !config.UseProxySubmit {
					emitLog(onLog, "info", "proxy submit disabled, skip proxy retry")
				}
				backoff := time.Duration(randomBackoffMs(submitBackoffMinMs, submitBackoffMaxMs)) * time.Millisecond
				emitLog(onLog, "warn", fmt.Sprintf("submit throttled, backoff %dms", backoff.Milliseconds()))
				if !sleepWithContext(ctx, backoff) {
					if proxyURL != "" && g.client != nil {
						if err := g.client.ClearProxy(); err != nil {
							emitLog(onLog, "warn", fmt.Sprintf("proxy clear failed: %v", err))
						} else {
							emitLog(onLog, "info", "proxy cleared (direct mode)")
						}
					}
					return nil, ctx.Err()
				}
				if proxyURL != "" {
					retryResult, retryErr := g.submitOnce(ctx, submitParams, onLog)
					if g.client != nil {
						if err := g.client.ClearProxy(); err != nil {
							emitLog(onLog, "warn", fmt.Sprintf("proxy clear failed: %v", err))
						} else {
							emitLog(onLog, "info", "proxy cleared (direct mode)")
						}
					}
					if retryErr != nil {
						emitLog(onLog, "error", fmt.Sprintf("submit retry error: %v", retryErr))
						return nil, nil
					}
					if retryResult != nil && (retryResult.Success || retryResult.Status) {
						unitName := fallback(config.UnitName, config.UnitID)
						depName := fallback(config.DepName, config.DepID)
						memberName := fallback(config.MemberName, config.MemberID)
						success := &GrabSuccess{
							UnitName:   unitName,
							DepName:    depName,
							DoctorName: docName,
							Date:       date,
							TimeSlot:   selected.Name,
							MemberName: memberName,
							URL:        retryResult.URL,
						}
						emitLog(onLog, "success", fmt.Sprintf("success: %s / %s / %s", unitName, depName, docName))
						return success, nil
					}
					retryMsg := ""
					if retryResult != nil {
						retryMsg = retryResult.Message
					}
					if retryMsg == "" {
						retryMsg = "submit retry failed"
					}
					emitLog(onLog, "warn", retryMsg)
				}
				return nil, nil
			}
			emitLog(onLog, "error", msg)
		}
	}
	if matchedDoctor > 0 && matchedSlots == 0 {
		emitLog(onLog, "info", fmt.Sprintf(
			"schedule stats: doctors=%d slots=%d passed_time=%d left>0=%d time_types=%s left_by_type=%s",
			matchedDoctor,
			totalSlots,
			checkedSlots,
			matchedSlots,
			formatCountMap(timeTypeCounts),
			formatCountMap(leftByType),
		))
		if len(samples) > 0 {
			emitLog(onLog, "info", fmt.Sprintf("schedule samples: %s", strings.Join(samples, " | ")))
		}
	}

	if isPrecise {
		if len(doctorSet) > 0 && matchedDoctor == 0 {
			emitLog(onLog, "warn", fmt.Sprintf("精细条件医生在 %s 无排班", date))
		} else if matchedDoctor > 0 && matchedSlots == 0 {
			if len(timeSet) > 0 && filteredByTime > 0 && checkedSlots == 0 {
				emitLog(onLog, "warn", fmt.Sprintf("精细条件时间段在 %s 无可用排班", date))
			} else if checkedSlots > 0 {
				emitLog(onLog, "warn", fmt.Sprintf("精细条件排班在 %s 无剩余号源", date))
			}
		}
	} else if matchedDoctor > 0 && matchedSlots == 0 && checkedSlots > 0 {
		emitLog(onLog, "warn", fmt.Sprintf("排班存在但 %s 无剩余号源", date))
	}
	return nil, nil
}

func formatCountMap(values map[string]int) string {
	if len(values) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := make([]string, 0, len(keys))
	for _, key := range keys {
		items = append(items, fmt.Sprintf("%s=%d", key, values[key]))
	}
	return strings.Join(items, ",")
}

func parseGrabConfig(input map[string]any) (GrabConfig, error) {
	if input == nil {
		return GrabConfig{}, errors.New("config is nil")
	}
	unitID := pickString(input, "unit_id")
	depID := pickString(input, "dep_id")
	memberID := pickString(input, "member_id")
	targetDates := pickStringSlice(input, "target_dates")
	if len(targetDates) == 0 {
		if value := pickString(input, "target_date"); value != "" {
			targetDates = []string{value}
		}
	}

	if unitID == "" || depID == "" || memberID == "" || len(targetDates) == 0 {
		return GrabConfig{}, errors.New("missing required fields (unit_id/dep_id/member_id/target_dates)")
	}

	doctorIDs := pickStringSlice(input, "doctor_ids")
	if len(doctorIDs) == 0 {
		if value := pickString(input, "doctor_id"); value != "" {
			doctorIDs = []string{value}
		}
	}
	timeTypes := pickStringSlice(input, "time_types")
	if len(timeTypes) == 0 {
		timeTypes = pickStringSlice(input, "time_slots")
	}

	return GrabConfig{
		UnitID:         unitID,
		UnitName:       pickString(input, "unit_name"),
		DepID:          depID,
		DepName:        pickString(input, "dep_name"),
		DoctorIDs:      doctorIDs,
		MemberID:       memberID,
		MemberName:     pickString(input, "member_name"),
		TargetDates:    targetDates,
		TimeTypes:      timeTypes,
		PreferredHours: pickStringSlice(input, "preferred_hours"),
		AddressID:      pickString(input, "addressId", "address_id"),
		Address:        pickString(input, "address"),
		StartTime:      pickString(input, "start_time"),
		UseServerTime:  pickBool(input, "use_server_time"),
		RetryInterval:  pickFloat(input, "retry_interval"),
		MaxRetries:     pickInt(input, "max_retries"),
		UseProxySubmit: pickBoolWithDefault(input, true, "use_proxy_submit", "proxy_submit_enabled"),
	}, nil
}

func pickTimeSlot(slots []TimeSlot, preferred []string) TimeSlot {
	if len(slots) == 0 {
		return TimeSlot{}
	}
	if len(preferred) > 0 {
		for _, p := range preferred {
			for _, slot := range slots {
				if slot.Name == p {
					return slot
				}
			}
		}
	}
	return slots[0]
}

func resolveAddress(config GrabConfig, detail *TicketDetail, onLog func(level, message string)) (string, string) {
	addressID := normalizeAddressID(config.AddressID)
	addressText := normalizeAddressText(config.Address)

	if addressID == "" || addressText == "" {
		addressID = normalizeAddressID(detail.AddressID)
		addressText = normalizeAddressText(detail.Address)
	}

	if (addressID == "" || addressText == "") && len(detail.Addresses) > 0 {
		for _, item := range detail.Addresses {
			candID := normalizeAddressID(item.ID)
			candText := normalizeAddressText(item.Text)
			if candID == "" || candText == "" {
				continue
			}
			addressID = candID
			addressText = candText
			emitLog(onLog, "warn", fmt.Sprintf("fallback address: %s", addressText))
			break
		}
	}

	return addressID, addressText
}

func normalizeAddressID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "0" || value == "-1" {
		return ""
	}
	return value
}

func normalizeAddressText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	placeholders := []string{
		"\u8bf7\u9009\u62e9",
		"\u8bf7\u586b\u5199",
		"\u8bf7\u8f93\u5165",
		"\u57ce\u5e02\u5730\u5740",
	}
	for _, p := range placeholders {
		if strings.Contains(value, p) {
			return ""
		}
	}
	return value
}

func waitUntil(ctx context.Context, targetTime time.Time, client *HealthClient, useServerTime bool, onLog func(level, message string)) {
	now := time.Now()
	// targetTime is already absolute

	offset := time.Duration(0)
	if useServerTime && client != nil {
		offset = calibrateTimeOffset(client, onLog)
	}
	adjusted := targetTime.Add(-offset)

	if adjusted.Before(now) || adjusted.Equal(now) {
		emitLog(onLog, "warn", fmt.Sprintf("target time already passed: %s", targetTime.Format("15:04:05")))
		return
	}
	wait := adjusted.Sub(now)
	emitLog(onLog, "info", fmt.Sprintf("waiting %0.1fs to start", wait.Seconds()))

	for {
		if ctx.Err() != nil {
			return
		}
		remaining := adjusted.Sub(time.Now())
		if remaining <= 2*time.Second {
			break
		}
		sleep := remaining - 2*time.Second
		if sleep > time.Second {
			sleep = time.Second
		}
		if !sleepWithContext(ctx, sleep) {
			return
		}
	}

	spinStart := time.Now()
	for {
		if ctx.Err() != nil {
			return
		}
		if time.Now().After(adjusted) {
			break
		}
		runtime.Gosched()
	}
	delay := time.Since(adjusted).Milliseconds()
	spin := time.Since(spinStart).Milliseconds()
	emitLog(onLog, "info", fmt.Sprintf("start trigger (spin=%dms delay=%dms)", spin, delay))
}

func (g *Grabber) submitOnce(ctx context.Context, params map[string]any, onLog func(level, message string)) (*SubmitOrderResult, error) {
	if g == nil || g.client == nil {
		return nil, errors.New("client not initialized")
	}
	if !g.applySubmitThrottle(ctx, onLog) {
		return nil, ctx.Err()
	}
	g.lastSubmitAt = time.Now()
	return g.client.SubmitOrder(params)
}

func (g *Grabber) applySubmitThrottle(ctx context.Context, onLog func(level, message string)) bool {
	if submitMinIntervalMs <= 0 {
		return true
	}
	if g == nil || g.lastSubmitAt.IsZero() {
		return true
	}
	elapsed := time.Since(g.lastSubmitAt)
	minInterval := time.Duration(submitMinIntervalMs) * time.Millisecond
	if elapsed >= minInterval {
		return true
	}
	wait := minInterval - elapsed
	emitLog(onLog, "info", fmt.Sprintf("submit throttle: wait %dms", wait.Milliseconds()))
	return sleepWithContext(ctx, wait)
}

func isTooFastMessage(message string) bool {
	message = strings.TrimSpace(message)
	if message == "" {
		return false
	}
	return strings.Contains(message, "太快") ||
		strings.Contains(message, "频繁") ||
		strings.Contains(message, "刷新")
}

func randomBackoffMs(minMs, maxMs int) int {
	if minMs <= 0 && maxMs <= 0 {
		return 0
	}
	if maxMs < minMs {
		maxMs = minMs
	}
	if maxMs == minMs {
		return maxMs
	}
	return minMs + rand.Intn(maxMs-minMs+1)
}

func calibrateTimeOffset(client *HealthClient, onLog func(level, message string)) time.Duration {
	start := time.Now()
	serverTime, err := client.GetServerDatetime()
	end := time.Now()
	if err != nil || serverTime == nil {
		emitLog(onLog, "warn", "server time unavailable")
		return 0
	}
	rtt := end.Sub(start)
	localMid := start.Add(rtt / 2)
	offset := serverTime.Sub(localMid)
	emitLog(onLog, "info", fmt.Sprintf("time offset %0.3fs", offset.Seconds()))
	return offset
}

func sleepWithContext(ctx context.Context, duration time.Duration) bool {
	if duration <= 0 {
		return true
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func emitLog(onLog func(level, message string), level, message string) {
	if onLog == nil {
		return
	}
	onLog(level, message)
}

func pickString(input map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := input[key]; ok {
			text := strings.TrimSpace(toString(value))
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func pickStringSlice(input map[string]any, key string) []string {
	value, ok := input[key]
	if !ok || value == nil {
		return nil
	}
	switch v := value.(type) {
	case []string:
		return trimStringSlice(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			text := strings.TrimSpace(toString(item))
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		text := strings.TrimSpace(toString(value))
		if text == "" {
			return nil
		}
		return []string{text}
	}
}

func pickBool(input map[string]any, key string) bool {
	value, ok := input[key]
	if !ok || value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

func pickBoolWithDefault(input map[string]any, defaultValue bool, keys ...string) bool {
	for _, key := range keys {
		value, ok := input[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case bool:
			return v
		case string:
			text := strings.TrimSpace(strings.ToLower(v))
			if text == "" {
				return defaultValue
			}
			if text == "1" || text == "true" || text == "yes" || text == "on" {
				return true
			}
			if text == "0" || text == "false" || text == "no" || text == "off" {
				return false
			}
		case int:
			return v != 0
		case int64:
			return v != 0
		case int32:
			return v != 0
		case float64:
			return v != 0
		case float32:
			return v != 0
		}
	}
	return defaultValue
}

func pickFloat(input map[string]any, key string) float64 {
	value, ok := input[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if v == "" {
			return 0
		}
		if f, err := parseFloat(v); err == nil {
			return f
		}
	}
	return 0
}

func pickInt(input map[string]any, key string) int {
	value, ok := input[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		if v == "" {
			return 0
		}
		if i, err := parseInt(v); err == nil {
			return i
		}
	}
	return 0
}

func trimStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func toSet(values []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		set[value] = struct{}{}
	}
	return set
}

func containsSet(set map[string]struct{}, value string) bool {
	if value == "" {
		return false
	}
	_, ok := set[value]
	return ok
}

func fallback(primary, secondary string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return secondary
}

func parseFloat(input string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(input), 64)
}

func parseInt(input string) (int, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(input), 10, 64)
	return int(value), err
}
