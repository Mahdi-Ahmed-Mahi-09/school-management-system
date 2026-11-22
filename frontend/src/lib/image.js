const ACCEPTED = ['image/jpeg', 'image/png', 'image/webp']
const MAX_INPUT_BYTES = 15 * 1024 * 1024

/**
 * Loads an image file, centre-crops it to a square and resizes it to `size`
 * pixels, returning a JPEG blob. This keeps uploads small (well under the
 * API's 1 MB limit) and strips EXIF metadata such as GPS location.
 */
export async function prepareProfilePhoto(file, size = 512) {
  if (!ACCEPTED.includes(file.type)) throw new Error('Use a JPEG, PNG or WebP image.')
  if (file.size > MAX_INPUT_BYTES) throw new Error('That image is too large. Choose one under 15 MB.')

  const bitmap = await createImageBitmap(file).catch(() => {
    throw new Error("That file couldn't be read as an image.")
  })
  const side = Math.min(bitmap.width, bitmap.height)
  const sx = (bitmap.width - side) / 2
  const sy = (bitmap.height - side) / 2
  const target = Math.min(size, side)

  const canvas = document.createElement('canvas')
  canvas.width = target
  canvas.height = target
  const ctx = canvas.getContext('2d')
  ctx.fillStyle = '#ffffff'
  ctx.fillRect(0, 0, target, target)
  ctx.imageSmoothingQuality = 'high'
  ctx.drawImage(bitmap, sx, sy, side, side, 0, 0, target, target)
  bitmap.close?.()

  return new Promise((resolve, reject) =>
    canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error('Could not process the image.'))), 'image/jpeg', 0.88),
  )
}
