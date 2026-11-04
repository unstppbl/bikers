import { useEffect, useRef } from 'react'
import { useI18n } from '../i18n'

const MAX_SOURCE_BYTES = 12 * 1024 * 1024

export function photoFileError(file: File) {
  const knownType = ['image/jpeg', 'image/png', 'image/webp'].includes(file.type)
  const unknownType = !file.type || file.type === 'application/octet-stream'
  if (!knownType && !(unknownType && /\.(jpe?g|png|webp)$/i.test(file.name))) return 'photoInvalidType'
  if (!file.size) return 'photoEmpty'
  if (file.size > MAX_SOURCE_BYTES) return 'photoTooLarge'
  return undefined
}

export function LocalPhotoPreview({ file }: { file: File }) {
  const { t } = useI18n()
  const image = useRef<HTMLImageElement>(null)
  useEffect(() => {
    const url = URL.createObjectURL(file)
    const element = image.current
    if (element) element.src = url
    return () => { element?.removeAttribute('src'); URL.revokeObjectURL(url) }
  }, [file])
  return <div className="photo-upload-preview"><img ref={image} alt={t('photoPreviewAlt')} /><span>{file.name}</span></div>
}
