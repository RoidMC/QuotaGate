<script setup lang="ts">
interface Props {
  modelValue?: string
  type?: 'text' | 'password' | 'email' | 'number' | 'tel' | 'url'
  placeholder?: string
  label?: string
  disabled?: boolean
  readonly?: boolean
  prefixIcon?: string
  suffixIcon?: string
  iconSize?: string
  iconColor?: string
  class?: string
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: '',
  type: 'text',
  placeholder: '',
  label: '',
  disabled: false,
  readonly: false,
  prefixIcon: '',
  suffixIcon: '',
  iconSize: '18px',
  iconColor: '#888'
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const handleInput = (event: Event) => {
  const target = event.target as HTMLInputElement
  emit('update:modelValue', target.value)
}
</script>

<template>
  <div class="ui-input-wrapper">
    <label v-if="label" class="ui-input-label">
      {{ label }}
    </label>
    <div class="ui-input-container">
      <span v-if="prefixIcon" class="ui-input-icon ui-input-icon--prefix">
        <Icon :name="prefixIcon" />
      </span>
      <input
        :type="type"
        :value="modelValue"
        :placeholder="placeholder"
        :disabled="disabled"
        :readonly="readonly"
        class="ui-input"
        @input="handleInput"
      />
      <span v-if="suffixIcon" class="ui-input-icon ui-input-icon--suffix">
        <Icon :name="suffixIcon" />
      </span>
    </div>
  </div>
</template>

<style scoped lang="scss">
.ui-input-wrapper {
  display: flex;
  flex-direction: column;
  gap: var(--input-label-gap, 6px);
  width: var(--input-wrapper-width, 100%);
}

.ui-input-label {
  font-size: var(--input-label-font-size, 14px);
  color: var(--input-label-color, var(--horizon-text-secondary));
}

.ui-input-container {
  display: flex;
  align-items: center;
  width: var(--input-width, 100%);
  height: var(--input-height, 2rem);
  border-radius: var(--input-border-radius, 0);
  background-color: var(--input-bg, var(--horizon-surface-bg-80));
  border: 1px solid var(--input-border, var(--horizon-border));
  transition: border-color 0.2s;
  box-sizing: border-box;

  &:focus-within {
    border-color: var(--input-focus-border, var(--horizon-text));
  }
}

.ui-input-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  font-size: var(--input-icon-size, 18px);
  color: var(--input-icon-color, var(--horizon-text-secondary));

  &--prefix {
    margin-left: var(--input-icon-gap, 8px);
    margin-right: var(--input-icon-gap, 8px);
    opacity: var(--input-prefix-opacity, 1);
  }

  &--suffix {
    margin-left: var(--input-icon-gap, 8px);
    margin-right: var(--input-icon-gap, 8px);
    opacity: var(--input-suffix-opacity, 1);
  }
}

.ui-input {
  width: 100%;
  height: 100%;
  padding: var(--input-padding, 0 0.5rem);
  font-size: var(--input-font-size, 0.8rem);
  color: var(--input-text, var(--horizon-text));
  border: none;
  outline: none;
  background: transparent;
  flex: 1;
  min-width: 0;
  box-sizing: border-box;

  &::placeholder {
    color: var(--input-placeholder-color, var(--horizon-text-secondary));
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  &:read-only {
    cursor: default;
  }
}
</style>
