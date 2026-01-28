import { ref, computed } from 'vue'
import {
    ListGrabProfiles,
    GetActiveGrabProfile,
    SetActiveGrabProfile,
    CreateGrabProfile,
    UpdateGrabProfile,
    DeleteGrabProfile,
    PatchGrabProfile
} from '../../wailsjs/go/main/App'
import { useLogger } from './useLogger'
import { useConfigManager } from './useConfigManager'

const profiles = ref([])
const activeProfileId = ref('')
const activeProfileName = ref('')
const loading = ref(false)

export function useMultiConfig() {
    const { pushLog, stringifyError } = useLogger()
    const { loadConfiguration } = useConfigManager()

    const loadProfiles = async () => {
        loading.value = true
        try {
            const list = await ListGrabProfiles()
            profiles.value = list || []

            // Find active profile
            const active = list.find(p => p.active)
            if (active) {
                activeProfileId.value = active.id
                activeProfileName.value = active.name
            }
        } catch (err) {
            pushLog('error', `加载配置列表失败: ${stringifyError(err)}`)
        } finally {
            loading.value = false
        }
    }

    const switchProfile = async (id) => {
        loading.value = true
        try {
            await SetActiveGrabProfile(id)
            await loadProfiles() // Refresh list state
            await loadConfiguration() // Load config content to UI
            pushLog('success', '已切换配置')
        } catch (err) {
            pushLog('error', `切换配置失败: ${stringifyError(err)}`)
        } finally {
            loading.value = false
        }
    }

    const createProfile = async (name, currentConfig) => {
        if (!name) return
        loading.value = true
        try {
            const id = await CreateGrabProfile(name, currentConfig)
            await loadProfiles()
            await loadConfiguration() // Refresh UI with new profile
            pushLog('success', `配置 "${name}" 已创建`)
            return id
        } catch (err) {
            pushLog('error', `创建配置失败: ${stringifyError(err)}`)
            throw err
        } finally {
            loading.value = false
        }
    }

    const deleteProfile = async (id) => {
        if (!id) return
        // Prevent deleting the last profile locally first for better UX
        if (profiles.value.length <= 1) {
            pushLog('warn', '无法删除最后一个配置')
            return
        }

        loading.value = true
        try {
            await DeleteGrabProfile(id)
            await loadProfiles()
            // If we deleted the active profile, the backend auto-switches
            // so we reload the configuration to reflect the change
            await loadConfiguration()
            pushLog('success', '配置已删除')
        } catch (err) {
            pushLog('error', `删除配置失败: ${stringifyError(err)}`)
        } finally {
            loading.value = false
        }
    }

    const updateProfileName = async (id, newName) => {
        // Implement if needed, currently reusing PatchGrabProfile
        try {
            // Note: Backend might need a specific API for renaming if name is not in Config map
            // For now, we assume name is metadata.
            // Actually, PatchGrabProfile only updates the `Config` struct inside the profile.
            // We might need a separate Rename API or UpdateGrabProfile should accept metadata.
            // Let's stick to Create/Delete for now.
        } catch (err) {
            // ...
        }
    }

    // Partial update helper
    const patchConfig = async (updates) => {
        if (!activeProfileId.value) return
        loading.value = true
        try {
            await PatchGrabProfile(activeProfileId.value, updates)
            pushLog('success', '配置已更新')
            await loadProfiles() // Refresh timestamp
        } catch (err) {
            pushLog('error', `更新配置失败: ${stringifyError(err)}`)
        } finally {
            loading.value = false
        }
    }

    return {
        profiles,
        activeProfileId,
        activeProfileName,
        loading,
        loadProfiles,
        switchProfile,
        createProfile,
        deleteProfile,
        patchConfig
    }
}
