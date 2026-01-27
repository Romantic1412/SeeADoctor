import { ref, computed, watch } from 'vue'
import {
    GetCities,
    GetHospitalsByCity,
    GetDepsByUnit,
    GetSchedule,
    GetUserState,
    SaveUserState
} from '../../wailsjs/go/main/App'
import { useLogger } from './useLogger'
import { useAuth } from './useAuth'

// Global Data State (Shared)
const cities = ref([])
const selectedCity = ref('')
const hospitals = ref([])
const deps = ref([])
const doctors = ref([])
const doctorPool = ref([])
const lastDepsUnitId = ref('')

// Global Selection State (Shared across views)
const unitId = ref('')
const depId = ref('')
const doctorId = ref('')
const memberId = ref('')
const addressId = ref('') // Added addressId/Text support if needed
const addressText = ref('')

// Loading States
const loadingCities = ref(false)
const loadingHospitals = ref(false)
const loadingDeps = ref(false)
const loadingDoctors = ref(false)
const loadingDoctorPool = ref(false)

export function useHospitalData() {
    const { pushLog, stringifyError } = useLogger()
    const { loggedIn, loginChecked } = useAuth()

    // Computed names helpers
    const selectedHospitalName = computed(() => {
        const match = hospitals.value.find((item) => item.id === unitId.value)
        return match ? match.name : ''
    })
    const selectedDepName = computed(() => {
        const match = deps.value.find((item) => item.id === depId.value)
        return match ? match.name : ''
    })
    const selectedDoctorName = computed(() => {
        const match = doctors.value.find((item) => item.id === doctorId.value)
        return match ? match.name : ''
    })

    // Save names to user state for Dashboard display
    const saveSelectionNames = async () => {
        try {
            const state = await GetUserState() || {}
            if (unitId.value) state.unit_id = unitId.value
            if (depId.value) state.dep_id = depId.value
            if (doctorId.value) state.doctor_id = doctorId.value
            if (memberId.value) state.member_id = memberId.value
            state.unit_name = selectedHospitalName.value
            state.dep_name = selectedDepName.value
            state.doctor_name = selectedDoctorName.value
            await SaveUserState(state)
        } catch (err) {
            // Silent fail, not critical
        }
    }

    // Watch for changes in selections and save names
    watch([unitId, selectedHospitalName], () => {
        if (unitId.value && selectedHospitalName.value) {
            saveSelectionNames()
        }
    })

    watch([depId, selectedDepName], () => {
        if (depId.value && selectedDepName.value) {
            saveSelectionNames()
        }
    })

    watch([doctorId, selectedDoctorName], () => {
        if (doctorId.value && selectedDoctorName.value) {
            saveSelectionNames()
        }
    })

    watch(memberId, () => {
        if (memberId.value) {
            saveSelectionNames()
        }
    })

    // Helper to ensure selections are valid (e.g. if loaded list changes)
    const applySelection = (targetRef, items) => {
        if (!Array.isArray(items) || items.length === 0) {
            targetRef.value = ''
            return
        }
        const current = String(targetRef.value || '')
        const matched = items.find((item) => item.id === current)
        if (matched) {
            targetRef.value = matched.id
        } else {
            // Don't auto-select first one implies manual choice, but App.vue did auto-select.
            // We'll stick to auto-select first for convenience.
            targetRef.value = items[0].id
        }
    }

    const canLoadByLogin = () => {
        return loginChecked.value && loggedIn.value
    }

    const loadCities = async (preferredCity) => {
        if (!canLoadByLogin()) {
            cities.value = []
            selectedCity.value = ''
            return
        }
        loadingCities.value = true
        try {
            const data = await GetCities()
            cities.value = Array.isArray(data) ? data : []
            if (cities.value.length > 0) {
                const match = cities.value.find(c => String(c.cityId) === String(preferredCity))
                selectedCity.value = match ? String(match.cityId) : String(cities.value[0].cityId)
            }
        } catch (err) {
            pushLog('error', `城市加载失败: ${stringifyError(err)}`)
        } finally {
            loadingCities.value = false
        }
    }

    const loadHospitals = async (cityId) => {
        if (!cityId) {
            hospitals.value = []
            return
        }

        if (!canLoadByLogin()) {
            hospitals.value = []
            return
        }

        loadingHospitals.value = true
        try {
            const data = await GetHospitalsByCity(String(cityId))
            hospitals.value = Array.isArray(data)
                ? data.map(item => ({
                    id: String(item.unit_id || ''),
                    name: String(item.unit_name || ''),
                })).filter(item => item.id && item.name)
                : []
            applySelection(unitId, hospitals.value)
        } catch (err) {
            pushLog('error', `医院加载失败: ${stringifyError(err)}`)
        } finally {
            loadingHospitals.value = false
        }
    }

    const loadDeps = async (unitIdVal) => {
        if (!unitIdVal) {
            deps.value = []
            return
        }

        if (!canLoadByLogin()) {
            deps.value = []
            return
        }

        const normalizedUnitId = String(unitIdVal)
        if (normalizedUnitId === lastDepsUnitId.value && deps.value.length > 0) {
            return
        }
        if (loadingDeps.value && normalizedUnitId === lastDepsUnitId.value) {
            return
        }

        loadingDeps.value = true
        lastDepsUnitId.value = normalizedUnitId
        try {
            pushLog('info', '正在加载科室...')
            const data = await GetDepsByUnit(String(unitIdVal))
            const items = []
            if (Array.isArray(data)) {
                data.forEach((item) => {
                    if (Array.isArray(item.childs)) {
                        item.childs.forEach((child) => {
                            const id = String(child.dep_id || child.id || '')
                            const name = String(child.dep_name || child.name || '')
                            if (id && name) items.push({ id, name })
                        })
                    } else {
                        const id = String(item.dep_id || item.id || '')
                        const name = String(item.dep_name || item.name || '')
                        if (id && name) items.push({ id, name })
                    }
                })
            }
            deps.value = items
            applySelection(depId, deps.value)
            if (items.length === 0) {
                pushLog('warn', '未获取到科室')
            } else {
                pushLog('success', `已加载 ${items.length} 个科室`)
            }
        } catch (err) {
            pushLog('error', `科室加载失败: ${stringifyError(err)}`)
        } finally {
            loadingDeps.value = false
        }
    }

    const loadDoctors = async (unitIdVal, depIdVal, dateValue) => {
        if (!unitIdVal || !depIdVal || !dateValue) {
            doctors.value = []
            return
        }
        loadingDoctors.value = true
        try {
            const data = await GetSchedule(String(unitIdVal), String(depIdVal), String(dateValue))
            doctors.value = Array.isArray(data)
                ? data.map(doc => ({
                    id: String(doc.doctor_id || ''),
                    name: String(doc.doctor_name || ''),
                    left: Number(doc.total_left_num || 0),
                    fee: String(doc.reg_fee || ''),
                    schedules: Array.isArray(doc.schedules) ? doc.schedules : []
                })).filter(doc => doc.id && doc.name)
                : []
        } catch (err) {
            pushLog('error', `排班加载失败: ${stringifyError(err)}`)
        } finally {
            loadingDoctors.value = false
        }
    }

    const buildDateRange = (baseDateStr, rangeDays) => {
        const parts = String(baseDateStr || '').split('-').map(Number)
        if (parts.length !== 3 || parts.some(Number.isNaN)) return []
        const base = new Date(parts[0], parts[1] - 1, parts[2])
        if (Number.isNaN(base.getTime())) return []

        const span = Math.max(0, parseInt(rangeDays, 10) || 0)
        const dates = []
        for (let i = -span; i <= span; i++) {
            const d = new Date(base)
            d.setDate(base.getDate() + i)
            const yy = d.getFullYear()
            const mm = String(d.getMonth() + 1).padStart(2, '0')
            const dd = String(d.getDate()).padStart(2, '0')
            dates.push(`${yy}-${mm}-${dd}`)
        }
        return dates
    }

    const loadDoctorPool = async (unitIdVal, depIdVal, baseDateStr, rangeDays = 0) => {
        if (!unitIdVal || !depIdVal || !baseDateStr) {
            doctorPool.value = []
            return
        }

        const dates = buildDateRange(baseDateStr, rangeDays)
        if (dates.length === 0) {
            doctorPool.value = []
            return
        }

        loadingDoctorPool.value = true
        try {
            const map = new Map()
            for (const date of dates) {
                let data = []
                try {
                    data = await GetSchedule(String(unitIdVal), String(depIdVal), String(date))
                } catch (err) {
                    pushLog('warn', `排班查询失败(${date}): ${stringifyError(err)}`)
                }
                if (!Array.isArray(data)) continue
                data.forEach((doc) => {
                    const id = String(doc?.doctor_id || '')
                    const name = String(doc?.doctor_name || '')
                    if (!id || !name) return
                    const fee = String(doc?.reg_fee || '')
                    const left = Number(doc?.total_left_num || 0)
                    const schedules = Array.isArray(doc?.schedules) ? doc.schedules : []
                    const slots = schedules.length
                    const entry = map.get(id) || {
                        id,
                        name,
                        fee,
                        left: 0,
                        scheduleDays: new Set(),
                        scheduleSlots: 0,
                        latestDate: ''
                    }
                    entry.name = entry.name || name
                    entry.fee = entry.fee || fee
                    entry.left += Number.isFinite(left) ? left : 0
                    entry.scheduleSlots += slots
                    entry.scheduleDays.add(date)
                    if (!entry.latestDate || date > entry.latestDate) {
                        entry.latestDate = date
                    }
                    map.set(id, entry)
                })
            }

            doctorPool.value = Array.from(map.values())
                .map(item => ({
                    id: item.id,
                    name: item.name,
                    fee: item.fee,
                    left: item.left,
                    scheduleDays: item.scheduleDays.size,
                    scheduleSlots: item.scheduleSlots,
                    latestDate: item.latestDate
                }))
                .sort((a, b) => {
                    if (b.scheduleDays !== a.scheduleDays) return b.scheduleDays - a.scheduleDays
                    return b.scheduleSlots - a.scheduleSlots
                })

            if (doctorPool.value.length === 0) {
                pushLog('warn', '未获取到候选医生')
            }
        } finally {
            loadingDoctorPool.value = false
        }
    }

    return {
        cities,
        selectedCity,
        hospitals,
        deps,
        doctors,
        doctorPool,
        loadingCities,
        loadingHospitals,
        loadingDeps,
        loadingDoctors,
        loadingDoctorPool,

        // Selection State
        unitId,
        depId,
        doctorId,
        memberId,
        addressId,
        addressText,
        selectedHospitalName,
        selectedDepName,
        selectedDoctorName,

        loadCities,
        loadHospitals,
        loadDeps,
        loadDoctors,
        loadDoctorPool,
        applySelection
    }
}
