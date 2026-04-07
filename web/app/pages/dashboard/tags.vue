<script setup lang="ts">
import { z } from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'
import type { KnownHostCandidate, TagMember } from '~/composables/useApi'

definePageMeta({ layout: 'dashboard', middleware: 'auth' })

const { getTags, createTag, deleteTag, getTagMembers, addTagMembers, removeTagMember, getKnownHosts } = useApi()
const toast = useToast()

const loading = ref(false)
const tags = ref<string[]>([])
const selectedTag = ref<string | null>(null)
const tagMembers = ref<TagMember[]>([])
const membersLoading = ref(false)

const showCreateModal = ref(false)
const createSchema = z.object({
  name: z.string().min(1, 'Tag name is required')
})
const createState = reactive({ name: '' })

const showAddMemberModal = ref(false)
const knownHostsLoading = ref(false)
const knownHosts = ref<KnownHostCandidate[]>([])
const selectedKnownHosts = ref<KnownHostCandidate[]>([])
const hostSearch = ref('')
const manualMembers = ref('')
let hostSearchTimer: ReturnType<typeof setTimeout> | null = null

async function loadTags() {
  loading.value = true
  const response = await getTags()
  if (response?.items) {
    tags.value = response.items
  }
  loading.value = false
}

async function loadKnownHosts(search = '') {
  knownHostsLoading.value = true
  const response = await getKnownHosts(search, 20)
  knownHosts.value = response?.known_hosts ?? []
  knownHostsLoading.value = false
}

function resetAddMembersModal() {
  selectedKnownHosts.value = []
  knownHosts.value = []
  hostSearch.value = ''
  manualMembers.value = ''
}

function memberLabel(member: Pick<TagMember, 'address' | 'host' | 'has_host_alias'>) {
  return member.has_host_alias && member.host
    ? `${member.host} (${member.address})`
    : member.address
}

function isKnownHostSelected(address: string) {
  return selectedKnownHosts.value.some(host => host.address === address)
}

function toggleKnownHost(host: KnownHostCandidate) {
  if (isKnownHostSelected(host.address)) {
    selectedKnownHosts.value = selectedKnownHosts.value.filter(candidate => candidate.address !== host.address)
    return
  }
  selectedKnownHosts.value = [...selectedKnownHosts.value, host]
}

async function handleCreateTag(event: FormSubmitEvent<z.output<typeof createSchema>>) {
  const response = await createTag(event.data.name)
  if (response) {
    toast.add({
      title: 'Tag created',
      description: `Tag "${event.data.name}" has been created`,
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
    showCreateModal.value = false
    createState.name = ''
    loadTags()
  }
}

async function handleDeleteTag(tagName: string) {
  const response = await deleteTag(tagName)
  if (response) {
    toast.add({
      title: 'Tag deleted',
      description: `Tag "${tagName}" has been deleted`,
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
    if (selectedTag.value === tagName) {
      selectedTag.value = null
      tagMembers.value = []
    }
    loadTags()
  }
}

async function selectTag(tagName: string) {
  selectedTag.value = tagName
  membersLoading.value = true
  const response = await getTagMembers(tagName)
  tagMembers.value = response?.tag_members ?? []
  membersLoading.value = false
}

async function handleAddMembers() {
  if (!selectedTag.value) {
    return
  }

  const members = Array.from(new Set([
    ...selectedKnownHosts.value.map(host => host.address),
    ...manualMembers.value
      .split('\n')
      .map(member => member.trim())
      .filter(Boolean)
  ]))

  if (members.length === 0) {
    toast.add({
      title: 'No members selected',
      description: 'Pick known hosts or enter at least one IP address or CIDR',
      color: 'warning',
      icon: 'i-lucide-alert-triangle'
    })
    return
  }

  const response = await addTagMembers(selectedTag.value, members)
  if (response) {
    toast.add({
      title: 'Members added',
      description: `${members.length} member(s) added to "${selectedTag.value}"`,
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
    tagMembers.value = response.tag_members ?? []
    showAddMemberModal.value = false
    resetAddMembersModal()
  }
}

async function handleRemoveMember(address: string) {
  if (!selectedTag.value) return

  const response = await removeTagMember(selectedTag.value, address)
  if (response) {
    toast.add({
      title: 'Member removed',
      description: `"${address}" has been removed from "${selectedTag.value}"`,
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
    selectTag(selectedTag.value)
  }
}

watch(showAddMemberModal, (open) => {
  if (hostSearchTimer) {
    clearTimeout(hostSearchTimer)
    hostSearchTimer = null
  }

  if (open) {
    loadKnownHosts('')
    return
  }

  resetAddMembersModal()
})

watch(hostSearch, (value) => {
  if (!showAddMemberModal.value) {
    return
  }
  if (hostSearchTimer) {
    clearTimeout(hostSearchTimer)
  }
  hostSearchTimer = setTimeout(() => {
    loadKnownHosts(value)
  }, 200)
})

onMounted(() => {
  loadTags()
})
</script>

<template>
  <UDashboardPanel>
    <template #header>
      <UDashboardNavbar title="Tag Management">
        <template #right>
          <UButton
            icon="i-lucide-plus"
            label="Create Tag"
            @click="showCreateModal = true"
          />
        </template>
      </UDashboardNavbar>
    </template>

    <template #body>
      <div class="p-6">
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <UCard>
            <template #header>
              <div class="flex items-center justify-between">
                <h3 class="font-semibold">Tags</h3>
                <UButton
                  icon="i-lucide-refresh-cw"
                  variant="ghost"
                  size="sm"
                  :loading="loading"
                  @click="loadTags"
                />
              </div>
            </template>

            <div v-if="loading" class="space-y-2">
              <USkeleton class="h-12 w-full" v-for="i in 5" :key="i" />
            </div>

            <div v-else-if="tags.length === 0" class="py-8">
              <UEmpty
                icon="i-lucide-tags"
                title="No tags yet"
                description="Create your first tag to get started"
              >
                <UButton
                  icon="i-lucide-plus"
                  label="Create Tag"
                  @click="showCreateModal = true"
                />
              </UEmpty>
            </div>

            <div v-else class="space-y-2">
              <div
                v-for="tag in tags"
                :key="tag"
                class="flex items-center justify-between p-3 rounded-lg cursor-pointer transition-colors"
                :class="selectedTag === tag ? 'bg-primary/10 border border-primary' : 'bg-muted/50 hover:bg-muted'"
                @click="selectTag(tag)"
              >
                <div class="flex items-center gap-2">
                  <UIcon name="i-lucide-tag" class="size-4" />
                  <span class="font-medium">{{ tag }}</span>
                </div>
                <UDropdownMenu
                  :items="[
                    [{ label: 'Delete', icon: 'i-lucide-trash', color: 'error' as const, onSelect: () => handleDeleteTag(tag) }]
                  ]"
                >
                  <UButton
                    icon="i-lucide-more-vertical"
                    variant="ghost"
                    size="sm"
                    @click.stop
                  />
                </UDropdownMenu>
              </div>
            </div>
          </UCard>

          <UCard>
            <template #header>
              <div class="flex items-center justify-between">
                <h3 class="font-semibold">
                  {{ selectedTag ? `Members of "${selectedTag}"` : 'Select a tag' }}
                </h3>
                <UButton
                  v-if="selectedTag"
                  icon="i-lucide-plus"
                  label="Add Members"
                  size="sm"
                  @click="showAddMemberModal = true"
                />
              </div>
            </template>

            <div v-if="!selectedTag" class="py-8">
              <UEmpty
                icon="i-lucide-users"
                title="No tag selected"
                description="Select a tag from the list to view its members"
              />
            </div>

            <div v-else-if="membersLoading" class="space-y-2">
              <USkeleton class="h-10 w-full" v-for="i in 5" :key="i" />
            </div>

            <div v-else-if="tagMembers.length === 0" class="py-8">
              <UEmpty
                icon="i-lucide-users"
                title="No members"
                description="Add members to this tag"
              >
                <UButton
                  icon="i-lucide-plus"
                  label="Add Members"
                  @click="showAddMemberModal = true"
                />
              </UEmpty>
            </div>

            <div v-else class="space-y-2">
              <div
                v-for="member in tagMembers"
                :key="member.address"
                class="flex items-center justify-between p-3 rounded-lg bg-muted/50"
              >
                <div class="flex items-center gap-3">
                  <UIcon :name="member.has_host_alias ? 'i-lucide-monitor' : 'i-lucide-network'" class="size-4" />
                  <div class="space-y-1">
                    <div class="font-medium text-sm">{{ memberLabel(member) }}</div>
                    <div v-if="member.has_host_alias" class="text-xs text-muted font-mono">{{ member.address }}</div>
                  </div>
                </div>
                <UButton
                  icon="i-lucide-x"
                  variant="ghost"
                  size="sm"
                  color="error"
                  @click="handleRemoveMember(member.address)"
                />
              </div>
            </div>
          </UCard>
        </div>
      </div>
    </template>
  </UDashboardPanel>

  <UModal v-model:open="showCreateModal" title="Create Tag" description="Enter a name for the new tag">
    <template #body>
      <UForm :schema="createSchema" :state="createState" class="space-y-4" @submit="handleCreateTag">
        <UFormField name="name" label="Tag Name">
          <UInput v-model="createState.name" placeholder="e.g., work, family, blocked" />
        </UFormField>
        <div class="flex justify-end gap-2">
          <UButton variant="ghost" label="Cancel" @click="showCreateModal = false" />
          <UButton type="submit" label="Create" />
        </div>
      </UForm>
    </template>
  </UModal>

  <UModal
    v-model:open="showAddMemberModal"
    title="Add Members"
    description="Pick known hosts by alias and optionally add raw IP addresses or CIDRs"
  >
    <template #body>
      <div class="space-y-4">
        <UFormField
          label="Known Hosts"
          description="Search aliased clients from DNS logs. Selected hosts are stored by address."
        >
          <div class="space-y-3">
            <UInput
              v-model="hostSearch"
              icon="i-lucide-search"
              placeholder="Search by alias or address"
            />

            <div v-if="selectedKnownHosts.length > 0" class="flex flex-wrap gap-2">
              <UBadge
                v-for="host in selectedKnownHosts"
                :key="host.address"
                color="primary"
                variant="subtle"
                class="cursor-pointer"
                @click="toggleKnownHost(host)"
              >
                {{ host.host }} ({{ host.address }})
              </UBadge>
            </div>

            <div class="rounded-lg border border-default max-h-56 overflow-y-auto">
              <div v-if="knownHostsLoading" class="p-3 space-y-2">
                <USkeleton class="h-10 w-full" v-for="i in 4" :key="i" />
              </div>
              <div v-else-if="knownHosts.length === 0" class="p-4 text-sm text-muted">
                No known hosts match this search.
              </div>
              <button
                v-for="host in knownHosts"
                :key="host.address"
                type="button"
                class="w-full px-3 py-3 text-left border-b border-default last:border-b-0 transition-colors"
                :class="isKnownHostSelected(host.address) ? 'bg-primary/10' : 'hover:bg-muted/50'"
                @click="toggleKnownHost(host)"
              >
                <div class="flex items-center justify-between gap-3">
                  <div class="space-y-1">
                    <div class="font-medium text-sm">{{ host.host }}</div>
                    <div class="font-mono text-xs text-muted">{{ host.address }}</div>
                  </div>
                  <UIcon
                    :name="isKnownHostSelected(host.address) ? 'i-lucide-check-circle-2' : 'i-lucide-plus-circle'"
                    class="size-4"
                  />
                </div>
              </button>
            </div>
          </div>
        </UFormField>

        <UFormField
          label="Manual Members"
          description="One raw IP address or CIDR per line. These are also stored by address."
        >
          <UTextarea
            v-model="manualMembers"
            placeholder="192.168.1.1&#10;10.0.0.0/24"
            :rows="5"
          />
        </UFormField>

        <div class="flex justify-end gap-2">
          <UButton variant="ghost" label="Cancel" @click="showAddMemberModal = false" />
          <UButton label="Add Members" @click="handleAddMembers" />
        </div>
      </div>
    </template>
  </UModal>
</template>
