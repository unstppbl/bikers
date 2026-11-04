import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { formatInTimeZone } from 'date-fns-tz'
import { request, safeExternalUrl } from '../api'
import { useI18n } from '../i18n'
import type { BikeModel, BikeModelImage } from '../types'

interface ModelIdentity { model_id: string; make: string; model: string; year: number }
interface Reference { model: BikeModel; image: BikeModelImage }

export function validReferenceYear(year: number, maximumYear: number) {
  return Number.isInteger(year) && year >= 1885 && year <= maximumYear
}

export function selectModelReference(models: readonly BikeModel[], modelId: string, year: number, maximumYear: number): Reference | undefined {
  if (!modelId || !validReferenceYear(year, maximumYear)) return undefined
  const model = models.find((item) => item.id === modelId)
  if (!model) return undefined
  const images = (model.images ?? []).filter((item) =>
    Number.isInteger(item.year_from) && Number.isInteger(item.year_to) &&
    item.year_from >= 1885 && item.year_from <= item.year_to && year >= item.year_from && year <= item.year_to &&
    Number.isSafeInteger(item.width) && item.width > 0 && Number.isSafeInteger(item.height) && item.height > 0 &&
    /^\/bike-models\/[a-z0-9][a-z0-9_-]*\.(?:jpg|jpeg|png|webp)$/i.test(item.url) &&
    !!safeExternalUrl(item.source_url) && !!safeExternalUrl(item.license_url) &&
    typeof item.author === 'string' && !!item.author.trim() && typeof item.license === 'string' && !!item.license.trim(),
  )
  const image = images.sort((left, right) => (left.year_to - left.year_from) - (right.year_to - right.year_from))[0]
  return image ? { model, image } : undefined
}

export function ModelReference({ bike, variant = 'detail', showUnavailable = false }: { bike: ModelIdentity; variant?: 'card' | 'detail' | 'form'; showUnavailable?: boolean }) {
  const { t } = useI18n()
  const [maximumYear] = useState(() => Number(formatInTimeZone(new Date(), 'Asia/Almaty', 'yyyy')) + 1)
  const validYear = validReferenceYear(bike.year, maximumYear)
  const queryText = `${bike.make.trim()} ${bike.model.trim()}`.trim()
  const query = useQuery({
    queryKey: ['bike-model-reference', bike.make, bike.model],
    queryFn: ({ signal }) => request<{ models: BikeModel[] }>(`/bike-models?query=${encodeURIComponent(queryText)}`, { signal }),
    enabled: !!bike.model_id && !!bike.make.trim() && !!bike.model.trim() && validYear,
    meta: { private: false },
    staleTime: 5 * 60_000,
    retry: false,
  })
  if (!bike.model_id || !validYear || !query.data || query.isError) return null
  const reference = selectModelReference(query.data.models, bike.model_id, bike.year, maximumYear)
  if (!reference) return showUnavailable && query.data.models.some((model) => model.id === bike.model_id) ? <p className="model-reference-unavailable">{t('modelReferenceUnavailable')}</p> : null
  return <ReferencePhoto key={`${bike.model_id}:${bike.year}:${reference.image.url}`} reference={reference} variant={variant} showUnavailable={showUnavailable} />
}

function ReferencePhoto({ reference, variant, showUnavailable }: { reference: Reference; variant: 'card' | 'detail' | 'form'; showUnavailable: boolean }) {
  const { t } = useI18n()
  const [failed, setFailed] = useState(false)
  if (failed) return showUnavailable ? <p className="model-reference-unavailable">{t('modelReferenceUnavailable')}</p> : null
  const { image, model } = reference
  const years = image.year_from === image.year_to ? String(image.year_from) : `${image.year_from}–${image.year_to}`
  return <figure className={`model-reference model-reference-${variant}`}><div className="model-reference-frame"><img src={image.url} width={image.width} height={image.height} alt={t('modelReferenceAlt', { make: model.make, model: model.model, years })} loading={variant === 'card' ? 'lazy' : 'eager'} decoding="async" referrerPolicy="no-referrer" onError={() => setFailed(true)} /></div><figcaption><p>{t('modelReferenceLabel', { years })}<span>{model.make} {model.model}</span></p><div className="model-reference-credits"><span>{t('modelReferenceAuthor', { author: image.author })}</span><a href={safeExternalUrl(image.source_url)} target="_blank" rel="noopener noreferrer">{t('modelReferenceSource')}</a><a href={safeExternalUrl(image.license_url)} target="_blank" rel="noopener noreferrer">{image.license}</a></div></figcaption></figure>
}
