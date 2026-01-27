<script setup>
import { computed } from 'vue'
import { useAuth } from '../../composables/useAuth'
import { useGrabTask } from '../../composables/useGrabTask'
import { useHospitalData } from '../../composables/useHospitalData'
import { useConfigManager } from '../../composables/useConfigManager'
import GlassCard from '../ui/GlassCard.vue'
import NeonButton from '../ui/NeonButton.vue'
import StatusBadge from '../ui/StatusBadge.vue'

const { 
  loggedIn, 
  loginChecked, 
  qrImageUrl, 
  qrStatus, 
  loginRunning, 
  toggleLogin, 
  userState,
  members
} = useAuth()

const { 
  grabRunning, 
  startGrab,
  stopGrab,
  targetDates,
  preferredHours,
  timeTypes,
  selectedScheduleId
} = useGrabTask()

// Status Derivations
const loginBtnLabel = computed(() => {
  if (loggedIn.value) return '已登录'
  return loginRunning.value ? '停止扫码' : '扫码登录'
})

const grabBtnVariant = computed(() => grabRunning.value ? 'danger' : 'primary')
const grabBtnLabel = computed(() => grabRunning.value ? '停止抢号' : '开始抢号')

// Simple summary
const configSummary = computed(() => {
  const parts = []
  if (userState.value?.unit_name) parts.push(userState.value.unit_name)
  if (userState.value?.dep_name) parts.push(userState.value.dep_name)
  return parts.join(' - ') || '暂无配置'
})

const proxySubmitEnabled = computed(() => {
  return userState.value?.proxy_submit_enabled !== false
})

  // Export selected IDs from useHospitalData
  const { 
    selectedCity, 
    hospitals, 
    deps, 
    doctors,
    unitId: selectedUnitId,
    depId: selectedDepId,
    doctorId: selectedDoctorId,
    memberId: selectedMemberId
  } = useHospitalData()

  const hasPreciseSelection = computed(() => {
    return Boolean(
      selectedDoctorId.value ||
      selectedScheduleId.value ||
      (Array.isArray(timeTypes.value) && timeTypes.value.length > 0) ||
      (Array.isArray(preferredHours.value) && preferredHours.value.length > 0)
    )
  })

  const executeGrab = async () => {
     // Build the config payload from shared state
     const config = {
        unit_id: selectedUnitId.value,
        dep_id: selectedDepId.value,
        member_id: selectedMemberId.value,
        // target_dates is already managed inside useGrabTask, 
        // but startGrab might expect us to pass them or it uses its own state.
        // Checking useGrabTask.js: startGrab takes configPayload.
        // It validates: unit_id, dep_id, member_id, target_dates.
        // So we MUST pass target_dates here if useGrabTask doesn't auto-include them from its state.
        // Let's check useGrabTask.js...
        // ... buildGrabConfig(rawConfig) checks rawConfig.target_dates.
        // So we need to pass it.
        target_dates: targetDates.value,
        use_proxy_submit: proxySubmitEnabled.value
     }

     if (hasPreciseSelection.value) {
        if (selectedDoctorId.value) {
           config.doctor_id = selectedDoctorId.value
        }
        if (Array.isArray(timeTypes.value) && timeTypes.value.length > 0) {
           config.time_types = timeTypes.value
        }
        if (Array.isArray(preferredHours.value) && preferredHours.value.length > 0) {
           config.preferred_hours = preferredHours.value
        }
     }
     
     // Note: Other optional fields (e.g. time_slots) can be added later if needed.
     // For now, minimal valid config.
     
     await startGrab(config)
  }

  const handleToggleGrab = () => {
    if (grabRunning.value) {
      stopGrab()
    } else {
      executeGrab()
    }
  }

  const { loadConfiguration, loading: configLoading } = useConfigManager()

  const handleLoadConfig = async () => {
    await loadConfiguration()
  }

  const emit = defineEmits(['navigate'])
</script>

<template>
  <div class="grid grid-cols-1 md:grid-cols-2 gap-6 pb-20">
    <!-- Status Card -->
    <GlassCard title="运行状态" className="h-full">
      <div class="flex flex-col items-center justify-center p-6 space-y-4">
        
        <div class="relative">
           <div :class="['flex items-center justify-center border-4 shadow-xl overflow-hidden transition-all duration-300', 
              loggedIn ? 'w-32 h-32 rounded-full border-emerald-500/50 shadow-emerald-500/20' : 'w-48 h-48 rounded-xl border-indigo-500/50 shadow-indigo-500/20 bg-white']">
              <img v-if="qrImageUrl && !loggedIn" :src="qrImageUrl" class="w-full h-full object-contain p-2" />
              <div v-else class="text-4xl font-bold text-white/20">
                 {{ loggedIn ? 'OK' : 'QR' }}
              </div>
           </div>
           <div v-if="loggedIn" class="absolute bottom-1 right-1 w-8 h-8 bg-emerald-500 rounded-full flex items-center justify-center border-2 border-slate-900">
              <svg class="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" /></svg>
           </div>
        </div>
        
        <div class="text-center">
           <h2 class="text-xl font-bold text-white mb-1">{{ loggedIn ? '已登录' : (qrStatus || '等待登录') }}</h2>
           <p class="text-sm text-slate-400">{{ loggedIn ? `就诊人: ${members.length} 位` : '请使用微信扫码登录' }}</p>
        </div>

        <NeonButton 
          :variant="loggedIn ? 'success' : 'primary'" 
          :disabled="loggedIn"
          @click="toggleLogin" 
          :loading="loginRunning && !qrImageUrl"
        >
           {{ loginBtnLabel }}
        </NeonButton>
      </div>
    </GlassCard>

    <!-- Task Control -->
    <GlassCard title="抢号控制" className="h-full">
      <div class="flex flex-col justify-between h-full space-y-6">
         <div>
            <div class="flex justify-between items-center mb-4">
               <span class="text-slate-400 text-sm">当前配置</span>
               <div class="flex gap-2 items-center">
                  <StatusBadge :variant="configSummary !== '暂无配置' ? 'info' : 'neutral'">{{ configSummary !== '暂无配置' ? 'Ready' : 'Empty' }}</StatusBadge>
                  <button
                     @click="handleLoadConfig"
                     :disabled="configLoading"
                     class="text-xs text-indigo-400 hover:text-indigo-300 transition-colors disabled:opacity-50"
                     title="从保存的配置加载"
                  >
                     <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                     </svg>
                  </button>
               </div>
            </div>
            <div class="p-4 bg-white/5 rounded-xl border border-white/5 space-y-2">
               <div class="flex justify-between">
                  <span class="text-slate-500 text-xs uppercase tracking-wider">医院</span>
                  <span class="text-slate-200 text-sm font-medium">{{ userState?.unit_name || '未选择' }}</span>
               </div>
               <div class="flex justify-between">
                  <span class="text-slate-500 text-xs uppercase tracking-wider">科室</span>
                  <span class="text-slate-200 text-sm font-medium">{{ userState?.dep_name || '未选择' }}</span>
               </div>
               <div class="flex justify-between">
                  <span class="text-slate-500 text-xs uppercase tracking-wider">医生</span>
                  <span class="text-slate-200 text-sm font-medium">{{ userState?.doctor_name || '不限' }}</span>
               </div>
               <div class="flex justify-between">
                  <span class="text-slate-500 text-xs uppercase tracking-wider">日期</span>
                  <span class="text-slate-200 text-sm font-medium">{{ targetDates[0] || '未选择' }}</span>
               </div>
               <div class="flex justify-between">
                  <span class="text-slate-500 text-xs uppercase tracking-wider">时段</span>
                  <span class="text-slate-200 text-sm font-medium">{{ preferredHours.length > 0 ? preferredHours.join(', ') : (timeTypes.length > 0 ? timeTypes.map(t => t === 'am' ? '上午' : '下午').join(', ') : '不限') }}</span>
               </div>
               <div class="flex justify-between">
                  <span class="text-slate-500 text-xs uppercase tracking-wider">代理提交</span>
                  <span class="text-slate-200 text-sm font-medium">{{ proxySubmitEnabled ? '启用' : '禁用' }}</span>
               </div>
            </div>
         </div>
         
         <div class="flex flex-col gap-3">
             <NeonButton 
               size="lg" 
               :variant="grabBtnVariant" 
               @click="handleToggleGrab" 
               :loading="false" 
               block
             >
               {{ grabBtnLabel }}
             </NeonButton>
             <p class="text-center text-xs text-slate-500">
                请确保在配置面板中选择了正确的医院和日期
             </p>
         </div>
      </div>
    </GlassCard>
    
    <!-- Info -->
    <div class="md:col-span-2">
      <GlassCard title="快速指南" noPadding>
         <div class="grid grid-cols-1 sm:grid-cols-3 divide-y sm:divide-y-0 sm:divide-x divide-white/5">
            <div class="p-4 hover:bg-white/5 transition-colors cursor-pointer group" @click="$emit('navigate', 'dashboard')">
               <div class="w-10 h-10 rounded-lg bg-emerald-500/10 flex items-center justify-center mb-3 group-hover:scale-110 transition-transform">
                  <span class="text-emerald-400 font-bold">1</span>
               </div>
               <h4 class="font-medium text-slate-200 mb-1">扫码登录</h4>
               <p class="text-xs text-slate-500">使用微信扫码，确保状态显示“已登录”。</p>
            </div>
            <div class="p-4 hover:bg-white/5 transition-colors cursor-pointer group"  @click="$emit('navigate', 'config')">
               <div class="w-10 h-10 rounded-lg bg-indigo-500/10 flex items-center justify-center mb-3 group-hover:scale-110 transition-transform">
                  <span class="text-indigo-400 font-bold">2</span>
               </div>
               <h4 class="font-medium text-slate-200 mb-1">配置任务</h4>
               <p class="text-xs text-slate-500">选择医院、科室、医生以及就诊日期。</p>
            </div>
            <div class="p-4 hover:bg-white/5 transition-colors cursor-pointer group"  @click="$emit('navigate', 'logs')">
               <div class="w-10 h-10 rounded-lg bg-rose-500/10 flex items-center justify-center mb-3 group-hover:scale-110 transition-transform">
                  <span class="text-rose-400 font-bold">3</span>
               </div>
               <h4 class="font-medium text-slate-200 mb-1">启动抢号</h4>
               <p class="text-xs text-slate-500">点击开始，在日志监控中查看实时进度。</p>
            </div>
         </div>
      </GlassCard>
    </div>
  </div>
</template>
