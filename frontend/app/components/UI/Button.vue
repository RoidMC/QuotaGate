<script setup lang="ts">
interface Props {
  type?: 'button' | 'submit' | 'reset'
  disabled?: boolean
  loading?: boolean
  prefixIcon?: string
  suffixIcon?: string
  iconSize?: string
  iconColor?: string
  class?: string
}

const props = withDefaults(defineProps<Props>(), {
  type: 'button',
  disabled: false,
  loading: false,
  prefixIcon: '',
  suffixIcon: '',
  iconSize: '1.2rem',
  iconColor: 'currentColor'
})

const emit = defineEmits<{
  (e: 'click', event: MouseEvent): void
}>()

const slots = useSlots()
const hasContent = computed(() => !!slots.default?.())
const isImageUrl = (icon: string) =>
  icon.startsWith('/') || icon.startsWith('http') || icon.startsWith('data:')
</script>

<template>
  <button
    :type="type"
    :disabled="disabled || loading"
    class="ui-button"
    :class="class"
    @click="(e: MouseEvent) => !disabled && !loading && emit('click', e)"
  >
    <span v-if="loading" class="ui-button-loading">
      <Icon name="mdi:loading" />
    </span>
    <template v-else>
      <span v-if="prefixIcon" class="ui-button-icon">
        <img v-if="isImageUrl(prefixIcon)" :src="prefixIcon" />
        <Icon v-else :name="prefixIcon" />
      </span>
      <span v-if="hasContent" class="ui-button-text">
        <slot />
      </span>
      <span v-if="suffixIcon" class="ui-button-icon">
        <img v-if="isImageUrl(suffixIcon)" :src="suffixIcon" />
        <Icon v-else :name="suffixIcon" />
      </span>
    </template>
  </button>
</template>
