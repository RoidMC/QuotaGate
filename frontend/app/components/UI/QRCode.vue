<script setup lang="ts">
import QRCode from 'qrcode'

interface Props {
  value: string
  size?: number
  margin?: number
  color?: string
  bgColor?: string
  errorCorrectionLevel?: 'L' | 'M' | 'Q' | 'H'
  logo?: string
  logoSize?: number
  logoMargin?: number
  logoRadius?: number
}

const props = withDefaults(defineProps<Props>(), {
  size: 200,
  margin: 2,
  color: '#000000',
  bgColor: '#ffffff',
  errorCorrectionLevel: 'M',
  logoSize: 0.2,
  logoMargin: 2,
  logoRadius: 1
})

const svgContent = ref('')
const isClient = import.meta.client

const generateQR = async () => {
  if (!props.value || !isClient) return

  const svg = await QRCode.toString(props.value, {
    type: 'svg',
    width: props.size,
    margin: props.margin,
    color: {
      dark: props.color,
      light: props.bgColor
    },
    errorCorrectionLevel: props.errorCorrectionLevel
  })
  svgContent.value = svg
}

onMounted(generateQR)

watch(() => [props.value, props.size, props.color, props.bgColor], generateQR)
</script>

<template>
  <div
    v-if="svgContent"
    class="qr-code"
    :style="{ width: size + 'px', height: size + 'px' }"
  >
    <div class="qr-code-svg" v-html="svgContent" />
    <div
      v-if="logo"
      class="qr-code-logo"
      :style="{
        padding: logoMargin + 'px',
        backgroundColor: bgColor,
        borderRadius: logoRadius + 'px'
      }"
    >
      <img
        :src="logo"
        :style="{ width: size * logoSize + 'px', height: size * logoSize + 'px' }"
      />
    </div>
  </div>
</template>

<style lang="scss" scoped>
.qr-code {
  display: inline-block;
  position: relative;

  :deep(svg) {
    width: 100%;
    height: 100%;
  }
}

.qr-code-svg {
  width: 100%;
  height: 100%;
}

.qr-code-logo {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);

  img {
    display: block;
    object-fit: contain;
  }
}
</style>
