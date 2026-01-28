import { ref } from 'vue'
import { StartGrab, StopGrab, GetUserState, SaveUserState } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime'
import { useLogger } from './useLogger'

// Task Configuration State
const targetDates = ref([])
const grabRunning = ref(false)
const grabResult = ref(null)
const preferredHours = ref([])
const timeTypes = ref([])
const selectedScheduleId = ref('')

// Timing Configuration
const startTime = ref('')
const useServerTime = ref(true)
const preGrabTestEnabled = ref(true)
const preGrabTestOffsetSeconds = ref(60)

export function useGrabTask() {
    const { pushLog, stringifyError } = useLogger()

    // Config Persistence
    const saveTaskConfig = async () => {
        try {
            const state = await GetUserState() || {}
            state.preferred_hours = Array.isArray(preferredHours.value) ? preferredHours.value : []
            state.time_types = Array.isArray(timeTypes.value) ? timeTypes.value : []
            state.schedule_id = String(selectedScheduleId.value || '')
            state.target_dates = Array.isArray(targetDates.value) ? targetDates.value : []
            // Save timing configs (for legacy state file)
            state.start_time = startTime.value
            state.use_server_time = useServerTime.value
            state.pre_grab_test_enabled = preGrabTestEnabled.value
            state.pre_grab_test_offset_seconds = preGrabTestOffsetSeconds.value

            await SaveUserState(state)
        } catch (err) {
            pushLog('error', `保存配置失败: ${stringifyError(err)}`)
        }
    }

    const loadTaskConfig = async () => {
        try {
            const state = await GetUserState() || {}
            if (Array.isArray(state.preferred_hours)) {
                preferredHours.value = state.preferred_hours
            }
            if (Array.isArray(state.time_types)) {
                timeTypes.value = state.time_types
            }
            if (state.schedule_id) {
                selectedScheduleId.value = String(state.schedule_id)
            }
            if (Array.isArray(state.target_dates)) {
                targetDates.value = state.target_dates
            }
            if (state.start_time) startTime.value = state.start_time
            if (state.use_server_time !== undefined) useServerTime.value = state.use_server_time
            if (state.pre_grab_test_enabled !== undefined) preGrabTestEnabled.value = state.pre_grab_test_enabled
            if (state.pre_grab_test_offset_seconds !== undefined) preGrabTestOffsetSeconds.value = state.pre_grab_test_offset_seconds
        } catch (err) {
            pushLog('error', `加载配置失败: ${stringifyError(err)}`)
        }
    }

    // Date Management
    const addDateRange = (startDateStr, days) => { // 单选日期模式：仅使用起始日期
        const count = parseInt(days, 10)
        if (!count || count <= 0) return
        if (!startDateStr) {
            pushLog('warn', '请先选择日期')
            return
        }
        targetDates.value = [startDateStr]
        pushLog('warn', `当前为单选日期，已使用起始日期 ${startDateStr}`)
    }

    const addTargetDate = (dateStr) => {
        if (!dateStr) {
            pushLog('warn', '请先选择日期')
            return
        }
        targetDates.value = [dateStr]
        pushLog('success', `已设置日期 ${dateStr}`)
        saveTaskConfig()
    }

    const removeTargetDate = (dateStr) => {
        if (targetDates.value.length === 0) return
        if (targetDates.value[0] !== dateStr) return
        targetDates.value = []
        pushLog('warn', '已清除日期')
    }

    const clearTargetDates = () => {
        targetDates.value = []
        pushLog('warn', '已清空日期')
    }

    // Execution
    const buildGrabConfig = (rawConfig) => {
        // Validate required fields
        const errors = []
        if (!rawConfig.unit_id) errors.push('医院 ID')
        if (!rawConfig.dep_id) errors.push('科室 ID')
        if (!rawConfig.member_id) errors.push('就诊人 ID')
        if (!rawConfig.target_dates || rawConfig.target_dates.length === 0) errors.push('就诊日期')

        if (errors.length > 0) {
            throw new Error(`缺少必填项: ${errors.join(' / ')}`)
        }

        return rawConfig
    }

    const startGrab = async (configPayload) => {
        grabResult.value = null
        try {
            const validConfig = buildGrabConfig(configPayload)
            grabRunning.value = true
            // We pass the config to backend
            await StartGrab(validConfig)
            pushLog('info', '抢号任务已启动')
        } catch (err) {
            grabRunning.value = false
            pushLog('error', `启动抢号失败: ${stringifyError(err)}`)
        }
    }

    const stopGrab = async () => {
        try {
            await StopGrab()
            pushLog('warn', '正在停止任务...')
        } catch (err) {
            pushLog('error', `停止失败: ${stringifyError(err)}`)
        }
        // Note: grabRunning usually set to false by event 'grab-finished' or manual toggle
        // But immediate feedback is good
        grabRunning.value = false
    }

    const initGrabListeners = () => {
        EventsOn('grab-finished', (payload) => {
            grabRunning.value = false
            grabResult.value = payload || null
            if (payload?.success) {
                pushLog('success', payload?.message || '抢号完成')
            } else {
                pushLog('warn', payload?.message || '抢号失败')
            }
        })
    }

    return {
        targetDates,
        grabRunning,
        grabResult,
        preferredHours,
        timeTypes,
        selectedScheduleId,

        addDateRange,
        addTargetDate,
        removeTargetDate,
        clearTargetDates,
        startGrab,
        stopGrab,
        initGrabListeners,
        saveTaskConfig,
        loadTaskConfig
    }
}
