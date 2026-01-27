import { ref } from 'vue'
import { LoadGrabConfig, SaveGrabConfig } from '../../wailsjs/go/main/App'
import { useLogger } from './useLogger'
import { useHospitalData } from './useHospitalData'
import { useGrabTask } from './useGrabTask'

export function useConfigManager() {
    const { pushLog, stringifyError } = useLogger()
    const {
        unitId,
        depId,
        doctorId,
        memberId,
        selectedHospitalName,
        selectedDepName,
        selectedDoctorName
    } = useHospitalData()
    const {
        targetDates,
        preferredHours,
        timeTypes,
        selectedScheduleId
    } = useGrabTask()

    const loading = ref(false)

    const loadConfiguration = async () => {
        loading.value = true
        try {
            const config = await LoadGrabConfig()
            
            // Apply loaded configuration to current state
            if (config.unit_id) unitId.value = String(config.unit_id)
            if (config.dep_id) depId.value = String(config.dep_id)
            if (config.doctor_id) doctorId.value = String(config.doctor_id)
            if (config.member_id) memberId.value = String(config.member_id)
            
            if (Array.isArray(config.target_dates)) {
                targetDates.value = config.target_dates
            }
            if (Array.isArray(config.preferred_hours)) {
                preferredHours.value = config.preferred_hours
            }
            if (Array.isArray(config.time_types)) {
                timeTypes.value = config.time_types
            }
            if (config.schedule_id) {
                selectedScheduleId.value = String(config.schedule_id)
            }

            pushLog('success', '配置已加载')
            return config
        } catch (err) {
            pushLog('error', `加载配置失败: ${stringifyError(err)}`)
            throw err
        } finally {
            loading.value = false
        }
    }

    const saveConfiguration = async () => {
        loading.value = true
        try {
            const config = {
                unit_id: unitId.value,
                unit_name: selectedHospitalName.value,
                dep_id: depId.value,
                dep_name: selectedDepName.value,
                doctor_id: doctorId.value,
                doctor_name: selectedDoctorName.value,
                member_id: memberId.value,
                target_dates: Array.isArray(targetDates.value) ? targetDates.value : [],
                preferred_hours: Array.isArray(preferredHours.value) ? preferredHours.value : [],
                schedule_id: String(selectedScheduleId.value || ''),
                time_types: Array.isArray(timeTypes.value) ? timeTypes.value : []
            }

            await SaveGrabConfig(config)
            pushLog('success', '配置已保存')
        } catch (err) {
            pushLog('error', `保存配置失败: ${stringifyError(err)}`)
            throw err
        } finally {
            loading.value = false
        }
    }

    return {
        loading,
        loadConfiguration,
        saveConfiguration
    }
}
