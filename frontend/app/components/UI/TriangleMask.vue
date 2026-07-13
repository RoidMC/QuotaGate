<script setup lang="ts">
interface Props {
  position?: 'top-left' | 'top-right' | 'bottom-left' | 'bottom-right'
  size?: string
  backgroundColor?: string
  cursor?: string
}

const props = withDefaults(defineProps<Props>(), {
  position: 'top-right',
  size: 'var(--h-triangle-mask-size)',
  backgroundColor: 'var(--h-triangle-mask-bg)',
  cursor: 'var(--h-triangle-mask-cursor)'
})

const emit = defineEmits<{
  (e: 'click', event: MouseEvent): void
}>()

const clipPaths: Record<string, string> = {
  'top-left': 'polygon(0 0, 100% 0, 0 100%)',
  'top-right': 'polygon(100% 0, 100% 100%, 0 0)',
  'bottom-left': 'polygon(0 0, 0 100%, 100% 100%)',
  'bottom-right': 'polygon(100% 0, 100% 100%, 0 100%)'
}

const positions: Record<string, Record<string, string>> = {
  'top-left': { top: '0', left: '0' },
  'top-right': { top: '0', right: '0' },
  'bottom-left': { bottom: '0', left: '0' },
  'bottom-right': { bottom: '0', right: '0' }
}

const maskStyle = {
  width: props.size,
  height: props.size,
  backgroundColor: props.backgroundColor,
  clipPath: clipPaths[props.position],
  cursor: props.cursor,
  ...positions[props.position]
}

const handleClick = (event: MouseEvent) => {
  emit('click', event)
}
</script>

<template>
  <div class="triangle-mask" :style="maskStyle" @click="handleClick">
    <div class="triangle-mask-content">
      <slot />
    </div>
  </div>
</template>

<style scoped lang="scss">
.triangle-mask {
  position: absolute;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  transition: var(--h-triangle-mask-transition);

  &:hover {
    opacity: var(--h-triangle-mask-hover-opacity);
  }

  &-content {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 100%;
    pointer-events: none;
  }
}
</style>
