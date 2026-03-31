<script setup lang="ts">
import { z } from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'

definePageMeta({ layout: 'dashboard', middleware: 'auth' })

const { getTags, createTag, deleteTag, getTagMembers, addTagMembers, removeTagMember } = useApi()
const toast = useToast()

const loading = ref(false)
const tags = ref<string[]>([])
const selectedTag = ref<string | null>(null)
const tagMembers = ref<string[]>([])
const membersLoading = ref(false)

// Create tag modal
const showCreateModal = ref(false)
const createSchema = z.object({
  name: z.string().min(1, 'Tag name is required')
})
const createState = reactive({ name: '' })

// Add member modal
const showAddMemberModal = ref(false)
const addMemberSchema = z.object({
  members: z.string().min(1, 'At least one member is required')
})
const addMemberState = reactive({ members: '' })

async function loadTags() {
  loading.value = true
  const response = await getTags()
  if (response?.items) {
    tags.value = response.items
  }
  loading.value = false
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
  if (response?.items) {
    tagMembers.value = response.items
  } else {
    tagMembers.value = []
  }
  membersLoading.value = false
}

async function handleAddMembers(event: FormSubmitEvent<z.output<typeof addMemberSchema>>) {
  if (!selectedTag.value) return
  
  const members = event.data.members.split('\n').map(m => m.trim()).filter(m => m)
  const response = await addTagMembers(selectedTag.value, members)
  if (response) {
    toast.add({
      title: 'Members added',
      description: `${members.length} member(s) added to "${selectedTag.value}"`,
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
    showAddMemberModal.value = false
    addMemberState.members = ''
    selectTag(selectedTag.value)
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
          <!-- Tags List -->
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

          <!-- Tag Members -->
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
                :key="member"
                class="flex items-center justify-between p-3 rounded-lg bg-muted/50"
              >
                <div class="flex items-center gap-2">
                  <UIcon name="i-lucide-network" class="size-4" />
                  <span class="font-mono text-sm">{{ member }}</span>
                </div>
                <UButton
                  icon="i-lucide-x"
                  variant="ghost"
                  size="sm"
                  color="error"
                  @click="handleRemoveMember(member)"
                />
              </div>
            </div>
          </UCard>
        </div>
      </div>
    </template>
  </UDashboardPanel>

  <!-- Create Tag Modal -->
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

  <!-- Add Member Modal -->
  <UModal v-model:open="showAddMemberModal" title="Add Members" description="Add IP addresses or networks to this tag">
    <template #body>
      <UForm :schema="addMemberSchema" :state="addMemberState" class="space-y-4" @submit="handleAddMembers">
        <UFormField name="members" label="Members" description="One IP address or CIDR per line">
          <UTextarea
            v-model="addMemberState.members"
            placeholder="192.168.1.1&#10;10.0.0.0/24"
            :rows="5"
          />
        </UFormField>
        <div class="flex justify-end gap-2">
          <UButton variant="ghost" label="Cancel" @click="showAddMemberModal = false" />
          <UButton type="submit" label="Add Members" />
        </div>
      </UForm>
    </template>
  </UModal>
</template>
