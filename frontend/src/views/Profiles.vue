<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="text-3xl font-bold">Profiles</h1>
        <p class="mt-1 text-base-content/70">Define reusable server and tool scopes for MCP clients and pinned agent tokens.</p>
      </div>
      <div class="flex gap-2">
        <button class="btn btn-outline" :disabled="loading" @click="load">
          <span v-if="loading" class="loading loading-spinner loading-sm" />
          Refresh
        </button>
        <button class="btn btn-primary" @click="newProfile">+ New Profile</button>
      </div>
    </div>

    <div v-if="error" class="alert alert-error">{{ error }}</div>
    <div v-if="notice" class="alert alert-success">{{ notice }}</div>

    <div v-if="loading && !loaded" class="py-12 text-center">
      <span class="loading loading-spinner loading-lg" />
      <p class="mt-3">Loading profiles and discovered tools…</p>
    </div>

    <div v-else class="grid gap-6 lg:grid-cols-[18rem_1fr]">
      <div class="card bg-base-100 shadow-md">
        <div class="card-body p-3">
          <h2 class="px-3 pb-2 text-xs font-semibold uppercase tracking-wider text-base-content/50">Configured profiles</h2>
          <button
            v-for="profile in profiles"
            :key="profile.name"
            class="mb-1 w-full rounded-lg px-3 py-3 text-left hover:bg-base-200"
            :class="selectedName === profile.name ? 'bg-base-200 ring-1 ring-primary/40' : ''"
            @click="selectProfile(profile.name)"
          >
            <div class="flex items-center justify-between gap-2">
              <span class="font-medium truncate">{{ profile.name }}</span>
              <span class="badge badge-sm badge-ghost">{{ profileToolCount(profile) }}</span>
            </div>
            <div class="mt-1 text-xs text-base-content/60">{{ profile.servers.length }} servers</div>
          </button>
          <div v-if="profiles.length === 0" class="px-3 py-5 text-sm text-base-content/60">No profiles yet.</div>
        </div>
      </div>

      <div v-if="draft" class="card bg-base-100 shadow-md">
        <div class="card-body gap-5">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 class="card-title">{{ editing ? 'Edit profile' : 'New profile' }}</h2>
              <p class="text-sm text-base-content/60">Select servers, then turn individual discovered tools on or off.</p>
            </div>
            <button v-if="editing" class="btn btn-sm btn-ghost text-error" @click="removeProfile">Delete</button>
          </div>

          <label class="form-control max-w-md">
            <span class="label-text mb-1 font-medium">Profile name</span>
            <input v-model.trim="draft.name" class="input input-bordered" :disabled="editing" placeholder="e.g. readonly" />
            <span class="label-text-alt mt-1 text-base-content/60">Lowercase letters, numbers, hyphens, and underscores.</span>
          </label>

          <div>
            <div class="mb-2 flex items-center justify-between">
              <h3 class="font-semibold">Servers and tools</h3>
              <span class="text-xs text-base-content/60">{{ selectedToolCount }} selected tools</span>
            </div>
            <div class="space-y-3">
              <div v-for="server in servers" :key="server.name" class="rounded-lg border border-base-300">
                <label class="flex cursor-pointer items-center gap-3 p-3 hover:bg-base-200/50">
                  <input v-model="draft.servers" type="checkbox" class="checkbox checkbox-primary" :value="server.name" />
                  <span class="font-medium">{{ server.title || server.name }}</span>
                  <span class="text-xs text-base-content/50">{{ toolList(server.name).length }} discovered tools</span>
                  <span class="ml-auto text-xs text-base-content/60">{{ server.connected ? 'Connected' : 'Offline' }}</span>
                </label>
                <div v-if="draft.servers.includes(server.name)" class="border-t border-base-300 bg-base-200/20 p-3">
                  <div v-if="loadingTools[server.name]" class="text-sm text-base-content/60">Loading tools…</div>
                  <div v-else-if="toolList(server.name).length === 0" class="text-sm text-base-content/60">No discovered tools. Reconnect or discover this server first.</div>
                  <div v-else class="grid gap-x-4 gap-y-1 sm:grid-cols-2 lg:grid-cols-3">
                    <label v-for="tool in toolList(server.name)" :key="tool.name" class="flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-sm hover:bg-base-200">
                      <input
                        type="checkbox"
                        class="checkbox checkbox-sm checkbox-primary"
                        :checked="isToolSelected(server.name, tool.name)"
                        @change="toggleTool(server.name, tool.name, ($event.target as HTMLInputElement).checked)"
                      />
                      <span class="truncate" :title="tool.name">{{ tool.name }}</span>
                      <span
                        class="badge badge-xs shrink-0"
                        :class="toolRiskBadgeClass(tool)"
                        :title="`Recommended permission tier: ${toolRiskLabel(tool)}`"
                      >{{ toolRiskLabel(tool) }}</span>
                    </label>
                  </div>
                  <div class="mt-2 flex gap-2">
                    <button class="btn btn-xs btn-ghost" @click="selectAll(server.name)">Select all</button>
                    <button class="btn btn-xs btn-ghost" @click="selectNone(server.name)">Select none</button>
                    <button class="btn btn-xs btn-success btn-outline" @click="selectByRisk(server.name, 'read')">Select read-only</button>
                    <button class="btn btn-xs btn-warning btn-outline" @click="selectByRisk(server.name, 'write')">Select write</button>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="card-actions justify-end">
            <button class="btn" @click="draft = null">Cancel</button>
            <button class="btn btn-primary" :disabled="saving" @click="saveProfile">
              <span v-if="saving" class="loading loading-spinner loading-sm" />
              Save Profile
            </button>
          </div>
        </div>
      </div>
      <div v-else class="card bg-base-100 shadow-md">
        <div class="card-body items-center justify-center py-16 text-center">
          <h2 class="text-xl font-semibold">Choose a profile</h2>
          <p class="text-base-content/60">Select a profile to edit its server and tool scope, or create a new one.</p>
          <button class="btn btn-primary" @click="newProfile">Create Profile</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import api from '@/services/api'
import { useSystemStore } from '@/stores/system'
import type { ProfileSummary, Server, Tool } from '@/types'

type Draft = { name: string; servers: string[]; tools: Record<string, string[]> }
const profiles = ref<ProfileSummary[]>([])
const systemStore = useSystemStore()
const servers = ref<Server[]>([])
const tools = reactive<Record<string, Tool[]>>({})
const loadingTools = reactive<Record<string, boolean>>({})
const selectedName = ref('')
const draft = ref<Draft | null>(null)
const loading = ref(false)
const loaded = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')
const editing = computed(() => !!draft.value && profiles.value.some(p => p.name === draft.value?.name))
const selectedToolCount = computed(() => draft.value ? draft.value.servers.reduce((n, s) => n + (draft.value?.tools[s]?.length ?? toolList(s).length), 0) : 0)

function toolList(server: string) { return tools[server] ?? [] }
function profileToolCount(profile: ProfileSummary) { return profile.servers.reduce((n, s) => n + (profile.tools?.[s]?.length ?? toolList(s).length), 0) }
function toolRisk(tool: Tool): 'read' | 'write' | 'destructive' {
  if (tool.annotations?.destructiveHint) return 'destructive'
  if (tool.annotations?.readOnlyHint) return 'read'
  return 'write'
}
function toolRiskLabel(tool: Tool): string {
  const risk = toolRisk(tool)
  return risk === 'read' ? 'read-only' : risk === 'destructive' ? 'dangerous' : 'write'
}
function toolRiskBadgeClass(tool: Tool): string {
  const risk = toolRisk(tool)
  return risk === 'read' ? 'badge-success' : risk === 'destructive' ? 'badge-error' : 'badge-warning'
}

async function loadTools(server: string) {
  if (tools[server] || loadingTools[server]) return
  loadingTools[server] = true
  const response = await api.getServerTools(server)
  if (response.success && response.data) tools[server] = response.data.tools ?? []
  loadingTools[server] = false
}

async function load() {
  loading.value = true; error.value = ''; notice.value = ''
  const [profileResponse, serverResponse] = await Promise.all([api.getProfiles(), api.getServers()])
  if (!profileResponse.success || !serverResponse.success) error.value = profileResponse.error || serverResponse.error || 'Failed to load profiles'
  profiles.value = profileResponse.data?.profiles ?? []
  servers.value = (serverResponse.data?.servers ?? []).filter(s => s.enabled)
  await Promise.all(servers.value.map(s => loadTools(s.name)))
  loaded.value = true; loading.value = false
  if (!draft.value && profiles.value.length) selectProfile(profiles.value[0].name)
}

function selectProfile(name: string) {
  const p = profiles.value.find(item => item.name === name)
  if (!p) return
  selectedName.value = name
  draft.value = { name: p.name, servers: [...p.servers], tools: Object.fromEntries(Object.entries(p.tools ?? {}).map(([s, list]) => [s, [...list]])) }
}
function newProfile() { selectedName.value = ''; draft.value = { name: '', servers: [], tools: {} } }
function isToolSelected(server: string, tool: string) { return draft.value?.tools[server]?.includes(tool) ?? !Object.prototype.hasOwnProperty.call(draft.value?.tools ?? {}, server) }
function toggleTool(server: string, tool: string, enabled: boolean) {
  if (!draft.value) return
  const current = draft.value.tools[server] ?? toolList(server).map(t => t.name)
  draft.value.tools[server] = enabled ? [...new Set([...current, tool])] : current.filter(t => t !== tool)
}
function selectAll(server: string) { if (draft.value) draft.value.tools[server] = toolList(server).map(t => t.name) }
function selectNone(server: string) { if (draft.value) draft.value.tools[server] = [] }
function selectByRisk(server: string, risk: 'read' | 'write') {
  if (!draft.value) return
  draft.value.tools[server] = toolList(server).filter(tool => toolRisk(tool) === risk).map(tool => tool.name)
}

async function saveProfile() {
  if (!draft.value) return
  if (!/^[a-z0-9][a-z0-9_-]{0,62}$/.test(draft.value.name)) { error.value = 'Profile name must start with lowercase alphanumeric characters and contain only lowercase letters, numbers, hyphens, or underscores.'; return }
  if (!draft.value.servers.length) { error.value = 'Select at least one server.'; return }
  saving.value = true; error.value = ''
  const next = profiles.value.filter(p => p.name !== draft.value?.name).map(p => ({ name: p.name, servers: p.servers, ...(p.tools ? { tools: p.tools } : {}) }))
  const entry = { name: draft.value.name, servers: draft.value.servers, tools: Object.fromEntries(draft.value.servers.filter(s => Object.prototype.hasOwnProperty.call(draft.value?.tools ?? {}, s)).map(s => [s, draft.value?.tools[s]])) }
  const response = await api.patchConfig({ profiles: [...next, entry] })
  saving.value = false
  if (!response.success) {
    error.value = response.error || 'Failed to save profile'
    systemStore.addToast({ type: 'error', title: 'Profile Save Failed', message: error.value })
    return
  }
  notice.value = `Profile “${draft.value.name}” saved.`
  systemStore.addToast({ type: 'success', title: 'Profile Saved', message: `Profile “${draft.value.name}” was saved successfully.` })
  await load()
  selectedName.value = entry.name; selectProfile(entry.name)
}
async function removeProfile() {
  if (!draft.value || !confirm(`Delete profile “${draft.value.name}”?`)) return
  saving.value = true
  const response = await api.patchConfig({ profiles: profiles.value.filter(p => p.name !== draft.value?.name).map(p => ({ name: p.name, servers: p.servers, ...(p.tools ? { tools: p.tools } : {}) })) })
  saving.value = false
  if (!response.success) {
    error.value = response.error || 'Failed to delete profile'
    systemStore.addToast({ type: 'error', title: 'Profile Delete Failed', message: error.value })
    return
  }
  draft.value = null
  notice.value = 'Profile deleted.'
  systemStore.addToast({ type: 'success', title: 'Profile Deleted', message: 'The profile was deleted successfully.' })
  await load()
}

watch(() => draft.value?.servers, (value) => { value?.forEach(server => void loadTools(server)) }, { deep: true })
onMounted(() => void load())
</script>
