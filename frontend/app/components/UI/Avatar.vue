<script setup lang="ts">
interface Props {
  /** 头像 URL */
  src?: string
  /** 替代文本 */
  alt?: string
}

withDefaults(defineProps<Props>(), {
  src: '',
  alt: 'Avatar'
})

const emit = defineEmits<{
  click: []
  'upload-avatar': []
  'mouseenter': [event: MouseEvent]
  'mouseleave': [event: MouseEvent]
}>()

const handleClick = () => {
  emit('click')
}

const handleMouseEnter = (event: MouseEvent) => {
  emit('mouseenter', event)
}

const handleMouseLeave = (event: MouseEvent) => {
  emit('mouseleave', event)
}
</script>

<template>
  <div
    class="ui-avatar"
    @click="handleClick"
    @mouseenter="handleMouseEnter"
    @mouseleave="handleMouseLeave"
  >
    <img
      v-if="src"
      :src="src"
      :alt="alt"
      class="ui-avatar__img"
    />
    <slot v-else name="placeholder" />
    <slot />
  </div>
</template>

<style>
/* 结构性基础样式，确保图片填满容器 */
.ui-avatar__img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
</style>
