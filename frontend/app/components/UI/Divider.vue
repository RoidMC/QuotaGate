<script setup lang="ts">
interface Props {
  text?: string
  position?: 'left' | 'center' | 'right'
  color?: string
  textColor?: string
  thickness?: string
  margin?: string
  textPadding?: string
  fontSize?: string
}

const props = withDefaults(defineProps<Props>(), {
  text: '',
  position: 'center',
  color: 'var(--h-divider-color)',
  textColor: 'var(--h-divider-text-color)',
  thickness: 'var(--h-divider-thickness)',
  margin: 'var(--h-divider-margin)',
  textPadding: 'var(--h-divider-text-padding)',
  fontSize: 'var(--h-divider-text-font-size)'
})

const lineStyle = {
  backgroundColor: props.color,
  height: props.thickness
}

const textStyle = {
  color: props.textColor,
  padding: props.textPadding,
  fontSize: props.fontSize
}

const dividerStyle = {
  margin: props.margin
}
</script>

<template>
  <div class="divider" :style="dividerStyle">
    <div
      v-if="position !== 'left'"
      class="divider-line"
      :style="lineStyle"
    />
    <span
      v-if="text"
      class="divider-text"
      :style="textStyle"
    >
      {{ text }}
    </span>
    <div
      v-if="position !== 'right'"
      class="divider-line"
      :style="lineStyle"
    />
  </div>
</template>

<style scoped lang="scss">
.divider {
  display: flex;
  align-items: center;
  width: 100%;

  &-line {
    flex: 1;
    min-width: var(--h-divider-line-min-width);
  }

  &-text {
    white-space: nowrap;
  }
}
</style>
