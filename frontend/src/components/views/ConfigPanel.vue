<script setup>
import { computed, watch, ref, onMounted } from 'vue'
import { useHospitalData } from '../../composables/useHospitalData'
import { useAuth } from '../../composables/useAuth'
import { useGrabTask } from '../../composables/useGrabTask'
import { useLogger } from '../../composables/useLogger'
import { GetTicketDetail } from '../../../wailsjs/go/main/App'
import GlassCard from '../ui/GlassCard.vue'
import NeonButton from '../ui/NeonButton.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import Combobox from '../ui/Combobox.vue'

const { 
  cities, selectedCity, loadCities,
  hospitals, unitId, loadHospitals, loadingHospitals,
  deps, depId, loadDeps, loadingDeps,
  doctors, doctorId, loadDoctors, loadingDoctors,
  doctorPool, loadDoctorPool, loadingDoctorPool,
  memberId,
  loadingCities
} = useHospitalData()

const { members, loadMembers, loggedIn, loginChecked, userState, saveUserState, stateReady } = useAuth()

const { 
  targetDates,
  addTargetDate,
  clearTargetDates,
  preferredHours,
  timeTypes,
  selectedScheduleId,
  saveTaskConfig,
  loadTaskConfig
} = useGrabTask()

const { pushLog, stringifyError } = useLogger()

// Load saved configuration on mount
onMounted(() => {
  loadTaskConfig()
})

// Local UI state
const dateInput = ref('')
const timeSlots = ref([])
const timeSlotsLoading = ref(false)
const doctorRangeDays = ref(3)
const manualTimeInput = ref('')
const proxySubmitEnabled = ref(true)

watch(targetDates, (list) => {
  const value = Array.isArray(list) && list.length > 0 ? list[0] : ''
  if (dateInput.value !== value) dateInput.value = value
}, { immediate: true })

watch(
  () => userState.value?.proxy_submit_enabled,
  (value) => {
    proxySubmitEnabled.value = value !== false
  },
  { immediate: true }
)

watch(proxySubmitEnabled, (value) => {
  if (!stateReady.value) return
  if (!userState.value || typeof userState.value !== 'object') {
    userState.value = {}
  }
  const next = Boolean(value)
  if (userState.value.proxy_submit_enabled === next) return
  userState.value.proxy_submit_enabled = next
  saveUserState()
})

watch(dateInput, (value) => {
  const current = targetDates.value[0] || ''
  if (value === current) return
  if (value) {
    addTargetDate(value)
    if (!checkScheduleDate.value || checkScheduleDate.value === current) {
      checkScheduleDate.value = value
    }
    if (loginChecked.value && loggedIn.value && unitId.value && depId.value) {
      loadDoctors(unitId.value, depId.value, value)
    }
  } else {
    clearTargetDates()
  }
})

// Helpers to trigger loads when selection changes
watch(selectedCity, (newVal) => {
  if (!loginChecked.value || !loggedIn.value) return
  if (newVal) loadHospitals(newVal)
}, { immediate: true })

watch(unitId, (newVal) => {
  if (!loginChecked.value || !loggedIn.value) return
  if (newVal) loadDeps(newVal)
}, { immediate: true })

watch([loginChecked, loggedIn], ([checked, isLoggedIn]) => {
  if (!checked) return
  if (isLoggedIn) {
    if (cities.value.length === 0) loadCities()
    if (members.value.length === 0) loadMembers()
    return
  }
  cities.value = []
  selectedCity.value = ''
  hospitals.value = []
  unitId.value = ''
  deps.value = []
  depId.value = ''
  doctors.value = []
  doctorId.value = ''
  doctorPool.value = []
}, { immediate: true })

// Doctor loading depends on date too, complicated because we have multiple dates.
// Usually we check schedule for the 'first' target date or user manually checks.
// App.vue logic: loadDoctors(unitID, depID, dateValue).
// We'll add a 'Check Schedule' button or watchers.
const checkScheduleDate = ref('') 

const selectedDoctor = computed(() => {
  return doctors.value.find(item => item.id === doctorId.value) || null
})

const selectedSchedules = computed(() => {
  return Array.isArray(selectedDoctor.value?.schedules) ? selectedDoctor.value.schedules : []
})

const doctorScheduleMap = computed(() => {
  const map = new Map()
  doctors.value.forEach((doc) => {
    map.set(doc.id, doc)
  })
  return map
})

const hasPreciseSelection = computed(() => {
  return Boolean(
    doctorId.value ||
    selectedScheduleId.value ||
    (Array.isArray(preferredHours.value) && preferredHours.value.length > 0) ||
    (Array.isArray(timeTypes.value) && timeTypes.value.length > 0)
  )
})

watch(doctorId, () => {
  selectedScheduleId.value = ''
  preferredHours.value = []
  timeTypes.value = []
  timeSlots.value = []
})

const handleClearDate = () => {
  dateInput.value = ''
}

const handleCheckSchedule = () => {
    loadDoctors(unitId.value, depId.value, checkScheduleDate.value)
}

const handleBuildDoctorPool = async () => {
  syncTargetDateToSchedule()
  await loadDoctorPool(unitId.value, depId.value, checkScheduleDate.value, doctorRangeDays.value)
  await loadDoctors(unitId.value, depId.value, checkScheduleDate.value)
}
const handleSelectSchedule = async (slot) => {
  syncTargetDateToSchedule()
  const scheduleId = String(slot?.schedule_id || '')
  if (!scheduleId) return
  selectedScheduleId.value = scheduleId
  const timeType = String(slot?.time_type || '')
  timeTypes.value = timeType ? [timeType] : []
  preferredHours.value = []
  timeSlots.value = []

  if (!unitId.value || !depId.value) return
  timeSlotsLoading.value = true
  try {
    const detail = await GetTicketDetail(
      String(unitId.value),
      String(depId.value),
      scheduleId,
      String(memberId.value || '')
    )
    const times = detail?.times || detail?.time_slots || []
    timeSlots.value = Array.isArray(times)
      ? times.map(item => ({
        name: String(item?.name || '').trim(),
        value: String(item?.value || '').trim()
      })).filter(item => item.name)
      : []
    if (timeSlots.value.length === 0) {
      pushLog('warn', '未获取到具体时段')
    }
  } catch (err) {
    pushLog('error', `具体时段获取失败: ${stringifyError(err)}`)
  } finally {
    timeSlotsLoading.value = false
  }
  saveTaskConfig()
}

const togglePreferredHour = (name) => {
  const value = String(name || '').trim()
  if (!value) return
  const set = new Set(preferredHours.value)
  if (set.has(value)) {
    set.delete(value)
  } else {
    set.add(value)
  }
  preferredHours.value = Array.from(set)
  saveTaskConfig()
}

const clearPreferredHours = () => {
  preferredHours.value = []
  saveTaskConfig()
}

const addManualPreferredHour = () => {
  const value = String(manualTimeInput.value || '').trim()
  if (!value) return
  const set = new Set(preferredHours.value)
  set.add(value)
  preferredHours.value = Array.from(set)
  manualTimeInput.value = ''
  saveTaskConfig()
  pushLog('success', '时段已添加并保存')
}

const syncTargetDateToSchedule = () => {
  const scheduleDate = String(checkScheduleDate.value || '').trim()
  if (!scheduleDate) return
  const target = targetDates.value[0] || ''
  if (target === scheduleDate) return
  addTargetDate(scheduleDate)
  pushLog('warn', `排班查询日期与抢号日期不一致，已同步为 ${scheduleDate}`)
}

const clearPreciseSelection = () => {
  doctorId.value = ''
  selectedScheduleId.value = ''
  preferredHours.value = []
  timeTypes.value = []
  timeSlots.value = []
  manualTimeInput.value = ''
}

</script>

<template>
  <div class="space-y-6 pb-20">
    <!-- Main Config Grid -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 relative">

       <!-- Location & Hospital -->
       <GlassCard title="医院设置" className="relative z-30">
          <div class="space-y-4">
             <Combobox
                label="城市"
                v-model="selectedCity"
                :options="cities"
                key-field="cityId"
                label-field="name"
                placeholder="搜索城市..."
                :loading="loadingCities"
                :disabled="!loginChecked || !loggedIn"
             />
             
             <Combobox
                label="医院"
                v-model="unitId"
                :options="hospitals"
                key-field="id"
                label-field="name"
                placeholder="请选择或搜索医院..."
                :loading="loadingHospitals"
                :disabled="!loginChecked || !loggedIn || !selectedCity"
             />

             <Combobox
                label="科室"
                v-model="depId"
                :options="deps"
                key-field="id"
                label-field="name"
                placeholder="请选择或搜索科室..."
                :loading="loadingDeps"
                :disabled="!loginChecked || !loggedIn || !unitId"
             />
          </div>
       </GlassCard>

       <!-- Patient & Dates -->
       <GlassCard title="就诊信息" className="relative z-20">
          <div class="space-y-4">
             <div>
                <label class="block text-xs text-slate-400 mb-1.5 uppercase">就诊人</label>
                <select v-model="memberId" class="w-full bg-white/5 border border-white/10 rounded-lg px-4 py-2.5 text-sm focus:ring-2 focus:ring-indigo-500/50 outline-none">
                   <option v-if="!loggedIn" value="">请先登录</option>
                   <option v-else-if="members.length === 0" value="">无就诊人</option>
                   <option v-for="m in members" :key="m.id" :value="m.id">{{ m.name }} {{ m.certified ? '(实名)' : '' }}</option>
                </select>
             </div>

             <div class="pt-2 border-t border-white/5">
                <label class="block text-xs text-slate-400 mb-1.5 uppercase">日期 (单选)</label>
                <div class="flex gap-2 mb-2">
                   <input type="date" v-model="dateInput" class="glass-input flex-1 op-calendar" />
                   <NeonButton size="sm" variant="ghost" @click="handleClearDate" :disabled="targetDates.length === 0">清空</NeonButton>
                </div>
                <div class="text-xs text-slate-500">
                   当前选择：{{ targetDates[0] || '未选择' }}
                </div>
             </div>
          </div>
       </GlassCard>

       <!-- Submit Settings -->
       <GlassCard title="提交设置" className="lg:col-span-2 relative z-10">
          <div class="flex items-center justify-between bg-white/5 border border-white/10 rounded-xl px-4 py-3">
             <div>
                <div class="text-sm text-slate-200">代理提交</div>
                <div class="text-xs text-slate-500">仅在提交过频时使用代理重试</div>
             </div>
             <label class="relative inline-flex items-center cursor-pointer">
                <input type="checkbox" v-model="proxySubmitEnabled" class="sr-only peer" />
                <div class="w-11 h-6 bg-slate-700 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-indigo-500/50 rounded-full peer peer-checked:bg-emerald-500/70 transition-colors"></div>
                <div class="absolute left-1 top-1 w-4 h-4 bg-white rounded-full transition-transform peer-checked:translate-x-5"></div>
             </label>
          </div>
          <div class="mt-2 text-[11px] text-slate-500">
             关闭后提交仅走本地 IP，可能更容易触发“过于频繁”
          </div>
       </GlassCard>
       
       <!-- Doctor Schedule Check (Optional) -->
       <GlassCard title="排班查询 (可选)" className="lg:col-span-2 relative z-10">
          <div class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between p-1">
             <div class="flex-1 w-full sm:w-auto">
                <label class="block text-xs text-slate-400 mb-1.5">查询日期</label>
                <input type="date" v-model="checkScheduleDate" class="glass-input w-full" />
             </div>
             <div class="w-full sm:w-40">
                <label class="block text-xs text-slate-400 mb-1.5">医生范围(天)</label>
                <input type="number" v-model="doctorRangeDays" min="0" max="30" class="glass-input w-full" />
             </div>
             <NeonButton
               @click="handleBuildDoctorPool"
               :loading="loadingDoctorPool || loadingDoctors"
               :disabled="!loginChecked || !loggedIn || !unitId || !depId || !checkScheduleDate"
             >
                生成候选医生
             </NeonButton>
          </div>
          
          <div class="mt-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
             <div class="text-xs text-slate-500">
                选择医生/时段即启用精细抢号；未选择则按模糊抢号
             </div>
             <div class="flex items-center gap-2">
                <StatusBadge :variant="hasPreciseSelection ? 'success' : 'neutral'" dot>
                   {{ hasPreciseSelection ? '精细抢号' : '模糊抢号' }}
                </StatusBadge>
                <button
                  type="button"
                  class="text-[10px] text-slate-500 hover:text-slate-200 transition-colors"
                  @click="clearPreciseSelection"
                  :disabled="!hasPreciseSelection"
                >
                  清空精细条件
                </button>
             </div>
          </div>

          <div v-if="doctorPool.length > 0" class="mt-4 grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3 max-h-60 overflow-y-auto custom-scrollbar p-1">
             <div 
               v-for="doc in doctorPool" 
               :key="doc.id"
               @click="doctorId = doc.id"
               :class="['p-3 rounded-lg border cursor-pointer transition-all', doctorId === doc.id ? 'bg-emerald-500/10 border-emerald-500/50' : 'bg-white/5 border-white/5 hover:border-white/20']"
             >
                <div class="flex justify-between items-start">
                   <span class="font-medium text-sm text-slate-200">{{ doc.name }}</span>
                   <span class="text-xs text-emerald-400">¥{{ doc.fee }}</span>
                </div>
                <div class="text-xs text-slate-500 mt-1">
                   排班天数: {{ doc.scheduleDays }} · 排班场次: {{ doc.scheduleSlots }}
                </div>
                <div class="text-xs text-slate-500 mt-1">
                   最近排班: {{ doc.latestDate || '-' }}
                </div>
                <div v-if="doctorScheduleMap.get(doc.id)" class="text-xs text-emerald-400 mt-1">
                   当日余号: {{ doctorScheduleMap.get(doc.id)?.left || 0 }}
                </div>
             </div>
          </div>
          <div v-else-if="!loadingDoctorPool && checkScheduleDate" class="mt-4 text-center text-xs text-slate-500">
             暂无候选医生或未查询
          </div>

          <div v-if="doctorId && selectedSchedules.length > 0" class="mt-5">
             <label class="block text-xs text-slate-400 mb-2">选择时段</label>
             <div class="flex flex-wrap gap-2">
                <button
                  v-for="slot in selectedSchedules"
                  :key="slot.schedule_id || slot.time_type"
                  type="button"
                  @click="handleSelectSchedule(slot)"
                  :class="[
                    'px-3 py-1.5 rounded-lg text-xs border transition-colors',
                    String(selectedScheduleId) === String(slot.schedule_id)
                      ? 'bg-emerald-500/15 border-emerald-500/40 text-emerald-300'
                      : 'bg-white/5 border-white/10 text-slate-400 hover:text-slate-200'
                  ]"
                >
                  {{ slot.time_type_desc || (slot.time_type === 'am' ? '上午' : '下午') }}
                  <span class="ml-1 text-slate-500">余{{ slot.left_num || 0 }}</span>
                </button>
             </div>
          </div>
          <div v-else-if="doctorId" class="mt-3 text-xs text-slate-500">
             当前日期暂无该医生排班，无法选择时段
          </div>

          <div v-if="selectedScheduleId" class="mt-5">
             <div class="flex items-center justify-between">
                <label class="block text-xs text-slate-400">具体时段</label>
                <button type="button" class="text-[10px] text-slate-500 hover:text-slate-200" @click="clearPreferredHours">
                   清空
                </button>
             </div>
             <div v-if="timeSlotsLoading" class="mt-2 text-xs text-slate-500">正在加载时段...</div>
             <div v-else-if="timeSlots.length > 0" class="mt-3 flex flex-wrap gap-2">
                <button
                  v-for="slot in timeSlots"
                  :key="slot.value || slot.name"
                  type="button"
                  @click="togglePreferredHour(slot.name)"
                  :class="[
                    'px-3 py-1.5 rounded-lg text-xs border transition-colors',
                    preferredHours.includes(slot.name)
                      ? 'bg-indigo-500/20 border-indigo-500/40 text-indigo-300'
                      : 'bg-white/5 border-white/10 text-slate-400 hover:text-slate-200'
                  ]"
                >
                  {{ slot.name }}
                </button>
             </div>
             <div v-else class="mt-2 text-xs text-slate-500">暂无可选时段</div>
          </div>
          <div v-if="doctorId" class="mt-4">
             <label class="block text-[10px] text-slate-500 mb-1">手动添加时段 (如 08:30-09:00)</label>
             <div class="flex gap-2">
                <input type="text" v-model="manualTimeInput" class="glass-input flex-1 py-1 text-sm" placeholder="08:30-09:00" />
                <NeonButton size="sm" variant="ghost" @click="addManualPreferredHour" :disabled="!manualTimeInput">
                   添加
                </NeonButton>
                <NeonButton size="sm" variant="ghost" @click="clearPreferredHours" :disabled="preferredHours.length === 0">
                   清空
                </NeonButton>
             </div>
          </div>
       </GlassCard>

    </div>
  </div>
</template>

<style scoped>
.op-calendar {
  color-scheme: dark;
}
</style>
