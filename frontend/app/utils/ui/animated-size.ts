// Horizon UI Core - AnimatedSize

import { nextTick } from 'vue';
import type { Ref } from 'vue';

export interface AnimatedSizeOptions {
  /** 过渡时长，默认 300ms */
  duration?: number;
  /** 缓动函数，默认 ease */
  easing?: string;
  /** 是否同时过渡 opacity（需要在 CSS 中自行定义） */
}

/**
 * 让 wrapper 容器在子元素切换时平滑过渡宽高。
 * 与 Vue <Transition> 的 JS hooks 配合使用。
 */
export function useAnimatedSize<T extends HTMLElement>(
  wrapperRef: Ref<T | null>,
  options: AnimatedSizeOptions = {}
) {
  const { duration = 300, easing = 'ease' } = options;

  const applyTransition = () => {
    const el = wrapperRef.value;
    if (!el) return;
    el.style.transition = `width ${duration}ms ${easing}, height ${duration}ms ${easing}`;
    el.style.overflow = 'hidden';
  };

  const setSize = (width: number, height: number) => {
    const el = wrapperRef.value;
    if (!el) return;
    applyTransition();
    el.style.width = width > 0 ? `${width}px` : '';
    el.style.height = height > 0 ? `${height}px` : '';
  };

  const clearSize = () => {
    const el = wrapperRef.value;
    if (!el) return;
    el.style.width = '';
    el.style.height = '';
  };

  /** Vue Transition before-leave 钩子 */
  const beforeLeave = (el: HTMLElement) => {
    const rect = el.getBoundingClientRect();
    setSize(rect.width, rect.height);
  };

  /** Vue Transition enter 钩子 */
  const enter = (el: HTMLElement) => {
    nextTick(() => {
      const rect = el.getBoundingClientRect();
      setSize(rect.width, rect.height);
    });
  };

  /** Vue Transition after-enter 钩子 */
  const afterEnter = () => {
    clearSize();
  };

  return {
    beforeLeave,
    enter,
    afterEnter,
  };
}
