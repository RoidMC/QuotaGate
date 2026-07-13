<script setup lang="ts">
import {
    DialogClose,
    DialogContent,
    DialogOverlay,
    DialogPortal,
    DialogRoot,
    DialogTitle,
} from 'reka-ui'
import HoverCard from '~/components/UI/HoverCard.vue'

const props = defineProps<{
    open?: boolean
    translationFeedbackUrl?: string
}>()

const emit = defineEmits<{
    'update:open': [value: boolean]
}>()

const { locale, locales, setLocale } = useI18n()
const { width } = useMediaQuery()

const dialogMinWidth = computed(() => width.value < 768 ? '17.5rem' : '22rem')
const dialogMaxWidth = computed(() => width.value < 768 ? '90vw' : '22rem')

const searchQuery = ref('')

const filteredLocales = computed(() => {
    if (!searchQuery.value) return locales.value
    const query = searchQuery.value.toLowerCase()
    return locales.value.filter(lang =>
        (lang.name || '').toLowerCase().includes(query) ||
        (lang.code || '').toLowerCase().includes(query)
    )
})

const selectLanguage = (code: string) => {
    setLocale(code as any)
}

watch(() => props.open, (val) => {
    if (!val) searchQuery.value = ''
})
</script>

<template>
    <DialogRoot :open="props.open" @update:open="emit('update:open', $event)">
        <DialogPortal>
            <DialogOverlay class="language-dialog-overlay" />
            <!-- 阻止打开时自动聚焦搜索框 -->
            <DialogContent
                class="language-dialog-content"
                @open-auto-focus.prevent
                :style="{
                    minWidth: dialogMinWidth,
                    maxWidth: dialogMaxWidth,
                    transition: 'min-width 0.3s ease, max-width 0.3s ease'
                }"
            >
                <DialogTitle class="language-dialog-title">
                    <Icon name="mdi:translate" class="language-dialog-title-icon" />
                    <span>{{ $t('i18n-common-string.uni.language') }}</span>
                </DialogTitle>

                <div class="language-dialog-search">
                    <Icon name="tdesign:search" class="language-dialog-search-icon" />
                    <input
                        v-model="searchQuery"
                        type="text"
                        class="language-dialog-search-input"
                        :placeholder="$t('i18n-common-string.uni.search-language')"
                    />
                </div>

                <div class="language-dialog-list-wrapper">
                    <div class="language-dialog-list">
                        <button
                            v-for="lang in filteredLocales"
                            :key="lang.code"
                            class="language-dialog-item"
                            :class="{ 'language-dialog-item--active': locale === lang.code }"
                            @click="selectLanguage(lang.code)"
                        >
                            <Icon
                                :name="`cif:${lang.flag}`"
                                class="language-dialog-item-flag"
                            />
                            <span class="language-dialog-item-name">{{ lang.name }}</span>
                            <Icon
                                v-if="locale === lang.code"
                                name="tdesign:check"
                                class="language-dialog-item-check"
                            />
                        </button>
                        <Transition name="h-fade" mode="out-in">
                            <div v-if="filteredLocales.length === 0" key="empty" class="language-dialog-empty">
                                <Icon name="tdesign:search" class="language-dialog-empty-icon" />
                                <span>{{ $t('i18n-common-string.uni.no-results') }}</span>
                            </div>
                        </Transition>
                    </div>
                </div>

                <div class="language-dialog-footer">
                    <div class="language-dialog-footer-left">
                        <Icon name="tdesign:error-circle" class="language-dialog-footer-icon" />
                        <div class="language-dialog-footer-text">
                            <Transition name="h-fade" mode="out-in">
                                <span :key="locale">{{ $t('i18n-common-string.uni.translation-disclaimer') }}</span>
                            </Transition>
                        </div>
                    </div>
                    <HoverCard v-if="props.translationFeedbackUrl" :open-delay="100" :close-delay="50" side="top" align="center">
                        <a
                            :href="props.translationFeedbackUrl"
                            target="_blank"
                            class="language-dialog-footer-link"
                        >
                            <Icon name="tdesign:edit" />
                        </a>
                        <template #content>
                            <div class="language-dialog-footer-popup">
                                <span>{{ $t('i18n-common-string.uni.translation-feedback') }}</span>
                            </div>
                        </template>
                    </HoverCard>
                </div>

                <DialogClose class="language-dialog-close" aria-label="Close">
                    <Icon name="tdesign:close" />
                </DialogClose>
            </DialogContent>
        </DialogPortal>
    </DialogRoot>
</template>

<style lang="scss">
.language-dialog-overlay {
    background: var(--h-overlay);
    backdrop-filter: blur(var(--h-common-blur-xs));
    -webkit-backdrop-filter: blur(var(--h-common-blur-xs));
    position: fixed;
    inset: 0;
    z-index: 99998;
    animation: h-overlay-fade-in 0.2s ease-out;
}

.language-dialog-content {
    background: var(--h-surface-bg-80);
    backdrop-filter: blur(var(--h-common-blur-base)) saturate(180%);
    border: 1px solid var(--h-border);
    border-radius: var(--h-border-radius-base);
    padding: 1.5rem;
    transition: min-width 0.3s ease, max-width 0.3s ease;
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    z-index: 99999;
    box-shadow: 0 1.25rem 3.75rem var(--h-shadow);
    animation: h-dialog-scale-in 0.2s ease-out;
    outline: none;

    &[data-state="closed"] {
        animation: h-dialog-scale-out 0.15s ease-in;
    }
}

.language-dialog-title {
    font-size: var(--h-text-size-base);
    color: var(--h-text);
    margin: 0 0 1rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;

    &-icon {
        font-size: var(--h-icon-size-lg);
    }
}

.language-dialog-search {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    background: var(--h-interactive);
    border: 1px solid var(--h-border);
    border-radius: 0.5rem;
    padding: 0.5rem 0.75rem;
    margin-bottom: 0.75rem;
    transition: border-color 0.2s ease;

    &:focus-within {
        border-color: var(--h-border-strong);
    }

    &-icon {
        font-size: var(--h-icon-size-base);
        color: var(--h-text-secondary);
        flex-shrink: 0;
    }

    &-input {
        flex: 1;
        background: transparent;
        border: none;
        outline: none;
        color: var(--h-text);
        font-size: var(--h-text-size-sm);
        min-width: 0;

        &::placeholder {
            color: var(--h-text-secondary);
        }
    }
}

.language-dialog-footer {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 0.375rem;
    margin-top: 0.75rem;
    padding-top: 0.75rem;
    border-top: 1px solid var(--h-border);

    &-left {
        display: flex;
        align-items: center;
        gap: 0.375rem;
        flex: 1;
        min-width: 0;
    }

    &-icon {
        font-size: var(--h-icon-size-base);
        color: var(--h-text-secondary);
        flex-shrink: 0;
        line-height: 1;
    }

    &-text {
        flex: 1;
        min-width: 0;
        display: flex;
        align-items: center;

        span {
            font-size: var(--h-text-size-xs);
            color: var(--h-text-secondary);
            word-break: break-word;
        }
    }

    &-link {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: 1.5rem;
        height: 1.5rem;
        background: var(--h-interactive);
        border: none;
        border-radius: var(--h-border-radius-xs);
        color: var(--h-text-secondary);
        text-decoration: none;
        cursor: pointer;
        transition: all 0.2s ease;
        flex-shrink: 0;

        &:hover {
            background: var(--h-interactive-active);
            color: var(--h-text);
        }

        .iconify {
            font-size: var(--h-icon-size-base);
        }
    }

    &-popup {
        background: var(--h-surface-bg);
        backdrop-filter: blur(var(--h-common-blur-base));
        -webkit-backdrop-filter: blur(var(--h-common-blur-base));
        border: 1px solid var(--h-border);
        border-radius: var(--h-border-radius-sm);
        padding: 0.375rem 0.625rem;
        box-shadow: 0 0.5rem 1.5rem var(--h-shadow);
        z-index: 999999;

        span {
            font-size: var(--h-text-size-xs);
            color: var(--h-text);
            white-space: nowrap;
        }
    }
}

.language-dialog-list-wrapper {
    position: relative;
    border-radius: var(--h-border-radius-base);
}

.language-dialog-list {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    max-height: 15rem;
    overflow-y: auto;
    padding: 0.25rem;

    &::-webkit-scrollbar {
        width: 0.375rem;
    }

    &::-webkit-scrollbar-track {
        background: var(--h-interactive-active);
        border-radius: var(--h-border-radius-base);

    }

    &::-webkit-scrollbar-thumb {
        background: var(--h-interactive);
        border-radius: var(--h-border-radius-base);

        &:hover {
            background: var(--h-interactive-active);
        }
    }
}

.language-dialog-item {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem 1rem;
    background: transparent;
    border: 1px solid transparent;
    border-radius: 0.375rem;
    cursor: pointer;
    transition: all 0.2s ease;
    text-align: left;
    color: var(--h-text);
    width: 100%;

    &:hover {
        background: var(--h-interactive);
    }

    &--active {
        background: var(--h-interactive-active);
        border-color: var(--h-border);
    }

    &-flag {
        font-size: var(--h-icon-size-base);
        width: 1.5rem;
        height: 1.5rem;
        flex-shrink: 0;
    }

    &-name {
        font-size: var(--h-text-size-sm);
        font-weight: 500;
        flex: 1;
    }

    &-check {
        font-size: var(--h-icon-size-lg);
        color: var(--h-success);
    }
}

.language-dialog-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    padding: 1.5rem 0;
    color: var(--h-text-secondary);

    &-icon {
        font-size: var(--h-icon-size-lg);
    }
}

.language-dialog-close {
    position: absolute;
    top: 1rem;
    right: 1rem;
    width: 2.25rem;
    height: 2.25rem;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: 0.5rem;
    color: var(--h-text-secondary);
    cursor: pointer;
    transition: all 0.2s ease;

    &:hover {
        background: var(--h-interactive);
        color: var(--h-text);
    }

    .iconify {
        font-size: var(--h-icon-size-lg);
    }
}


</style>
