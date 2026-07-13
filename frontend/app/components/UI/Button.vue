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

<style scoped lang="scss">
.ui-button {
  width: var(--h-button-width);
  height: var(--h-button-height);
  padding: var(--h-button-padding);
  font-size: var(--h-button-font-size);
  border-radius: var(--h-button-border-radius);
  background-color: var(--h-button-bg);
  color: var(--h-button-text);
  border: 1px solid var(--h-button-border);
  gap: var(--h-button-icon-gap);

  display: inline-flex;
  align-items: center;
  justify-content: center;
  outline: none;
  cursor: pointer;
  transition: var(--h-button-transition);
  box-sizing: border-box;
  white-space: nowrap;
  user-select: none;

  &:hover:not(:disabled) {
    background-color: var(--h-button-hover-bg);
    color: var(--h-button-hover-text);
  }

  &:active:not(:disabled) {
    opacity: var(--h-button-active-opacity);
  }

  &:disabled {
    opacity: var(--h-button-disabled-opacity);
    cursor: not-allowed;
  }
}

.ui-button-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  font-size: var(--h-button-icon-size);
  color: var(--h-button-icon-color);

  img {
    width: var(--h-button-icon-size);
    height: var(--h-button-icon-size);
    object-fit: contain;
  }
}

.ui-button-text {
  display: flex;
  align-items: center;
}

.ui-button-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  animation: h-spin 1s linear infinite;
}
</style>
