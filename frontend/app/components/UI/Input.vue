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
  gap: var(--h-input-label-gap);
  width: var(--h-input-wrapper-width);
}

.ui-input-label {
  font-size: var(--h-input-label-font-size);
  color: var(--h-input-label-color);
}

.ui-input-container {
  display: flex;
  align-items: center;
  width: var(--h-input-width);
  height: var(--h-input-height);
  border-radius: var(--h-input-border-radius);
  background-color: var(--h-input-bg);
  border: 1px solid var(--h-input-border);
  transition: var(--h-input-transition);
  box-sizing: border-box;

  &:focus-within {
    border-color: var(--h-input-focus-border);
  }
}

.ui-input-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  font-size: var(--h-input-icon-size);
  color: var(--h-input-icon-color);

  &--prefix {
    margin-left: var(--h-input-icon-gap);
    margin-right: var(--h-input-icon-gap);
    opacity: var(--h-input-prefix-opacity);
  }

  &--suffix {
    margin-left: var(--h-input-icon-gap);
    margin-right: var(--h-input-icon-gap);
    opacity: var(--h-input-suffix-opacity);
  }
}

.ui-input {
  width: 100%;
  height: 100%;
  padding: var(--h-input-padding);
  font-size: var(--h-input-font-size);
  color: var(--h-input-text);
  border: none;
  outline: none;
  background: transparent;
  flex: 1;
  min-width: 0;
  box-sizing: border-box;

  &::placeholder {
    color: var(--h-input-placeholder-color);
  }

  &:disabled {
    opacity: var(--h-input-disabled-opacity);
    cursor: not-allowed;
  }

  &:read-only {
    cursor: default;
  }
}
</style>
