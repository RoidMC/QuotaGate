<script lang="ts" setup>
import { ref } from 'vue'

interface Props {
  title?: string
  icon?: string
  variant?: 'default' | 'elevated' | 'outlined'
  padding?: 'sm' | 'md' | 'lg'
  collapsible?: boolean
  defaultExpanded?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'default',
  padding: 'md',
  collapsible: false,
  defaultExpanded: true
})

const isExpanded = ref(props.defaultExpanded)

const toggleExpand = () => {
  if (props.collapsible) {
    isExpanded.value = !isExpanded.value
  }
}

const paddingVarMap = {
  sm: 'var(--h-panel-card-padding-sm)',
  md: 'var(--h-panel-card-padding-md)',
  lg: 'var(--h-panel-card-padding-lg)'
}
</script>

<template>
  <div :class="['panel-card', `panel-card--${variant}`, { 'panel-card--collapsible': collapsible }]">
    <div v-if="title || $slots.header" class="panel-card__header" @click="toggleExpand">
      <slot name="header">
        <div class="panel-card__header-content">
          <Icon v-if="icon" :name="icon" class="panel-card__icon" />
          <h3 v-if="title" class="panel-card__title">{{ title }}</h3>
        </div>
        <div class="panel-card__header-right">
          <div v-if="$slots.actions" class="panel-card__actions" @click.stop>
            <slot name="actions" />
          </div>
          <Icon
            v-if="collapsible"
            name="tdesign:chevron-down"
            class="panel-card__expand-icon"
            :class="{ 'panel-card__expand-icon--expanded': isExpanded }"
          />
        </div>
      </slot>
    </div>
    <div class="panel-card__divider" v-if="title || $slots.header" :class="{ 'panel-card__divider--collapsed': !isExpanded }" />
    <div class="panel-card__body-wrapper" :class="{ 'panel-card__body-wrapper--collapsed': !isExpanded }">
      <div class="panel-card__body">
        <div class="panel-card__body-inner" :style="{ padding: paddingVarMap[padding] }">
          <slot />
        </div>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
@use '@/assets/css/themes/mixins' as HorizonMixins;

.panel-card {
  width: var(--h-panel-card-width);
  background: var(--h-panel-card-bg);
  backdrop-filter: var(--h-panel-card-backdrop-filter);
  border: var(--h-panel-card-border);
  border-radius: var(--h-panel-card-border-radius);
  overflow: hidden;
  transition: var(--h-panel-card-transition);
  box-shadow: var(--h-panel-card-shadow);

  &:hover {
    box-shadow: var(--h-panel-card-hover-shadow);
  }

  &--elevated {
    box-shadow: var(--h-panel-card-elevated-shadow);

    &:hover {
      box-shadow: var(--h-panel-card-elevated-hover-shadow);
    }
  }

  &--outlined {
    border: var(--h-panel-card-outlined-border);
    box-shadow: var(--h-panel-card-outlined-shadow);

    &:hover {
      box-shadow: var(--h-panel-card-outlined-hover-shadow);
    }
  }

  &__header {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--h-panel-card-header-padding);
    @include HorizonMixins.decorator-horizon-transition-bar($position-map: (left: 0.4rem, top: 50%), $height: 50%);
  }

  &__header-content {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 0.2rem;
  }

  &__icon {
    font-size: var(--h-panel-card-icon-size);
    line-height: 1;
  }

  &__title {
    margin: 0;
    font-size: var(--h-panel-card-title-size);
    color: var(--h-panel-card-title-color);
  }

  &__actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  &__header-right {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  &__expand-icon {
    font-size: var(--h-panel-card-expand-icon-size);
    color: var(--h-panel-card-expand-icon-color);
    transition: transform 0.3s ease;

    &--expanded {
      transform: rotate(180deg);
    }
  }

  &--collapsible {
    .panel-card__header {
      cursor: pointer;
    }
  }

  &__divider {
    height: 1px;
    background: var(--h-panel-card-divider-bg);
    transition: opacity 0.3s ease;

    &--collapsed {
      opacity: 0;
    }
  }

  &__body-wrapper {
    display: grid;
    grid-template-rows: 1fr;
    transition: grid-template-rows 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    overflow: hidden;

    &--collapsed {
      grid-template-rows: 0fr;
    }
  }

  &__body {
    min-height: 0;
    overflow: hidden;
  }

  &__body-inner {
    // padding comes from CSS variables via inline style
  }
}
</style>