import { memo } from 'react'
import { Box } from '@mui/material'
import { UI } from '@/lib/design/tokens'
import type { DiffSegment } from '../text-diff'
import type { AnchoredFeedbackItem } from '../types'

const NO_ANCHORED_FEEDBACK: AnchoredFeedbackItem[] = []

const HighlightedDiffText = memo(function HighlightedDiffText({
  segments,
  anchors = NO_ANCHORED_FEEDBACK,
  ariaLabel,
}: {
  segments: DiffSegment[]
  anchors?: AnchoredFeedbackItem[]
  ariaLabel: string
}) {
  const renderedAnchors = new Set<string>()

  return (
    <Box
      role="group"
      aria-label={ariaLabel}
      sx={{ fontSize: 14, lineHeight: 1.9, color: UI.ink, whiteSpace: 'pre-wrap' }}
    >
      {segments.flatMap((segment, segmentIndex) => {
        const intersectingAnchors = anchors.filter(
          anchor => anchor.start < segment.end && anchor.end > segment.start,
        )
        const boundaries = new Set([segment.start, segment.end])
        for (const anchor of intersectingAnchors) {
          boundaries.add(Math.max(segment.start, anchor.start))
          boundaries.add(Math.min(segment.end, anchor.end))
        }
        const sortedBoundaries = [...boundaries].sort((a, b) => a - b)

        return sortedBoundaries.slice(0, -1).map((start, partIndex) => {
          const end = sortedBoundaries[partIndex + 1]
          const text = segment.text.slice(start - segment.start, end - segment.start)
          const activeAnchors = intersectingAnchors.filter(
            anchor => anchor.start < end && anchor.end > start,
          )
          const newAnchorIds = activeAnchors
            .filter(anchor => !renderedAnchors.has(anchor.anchorId))
            .map(anchor => anchor.anchorId)
          for (const anchor of activeAnchors) renderedAnchors.add(anchor.anchorId)
          const anchorId = newAnchorIds[0]
          const anchorProps = anchorId
            ? { id: anchorId, 'data-es-feedback-anchor': 'true', tabIndex: -1 }
            : {}
          const anchorMarkers = newAnchorIds.slice(1).map(anchorId => (
            <Box
              key={anchorId}
              component="span"
              id={anchorId}
              data-es-feedback-anchor="true"
              tabIndex={-1}
              sx={{ display: 'inline-block', width: 0, height: '1em', overflow: 'hidden', scrollMarginTop: '96px' }}
            />
          ))
          const anchorStyle = activeAnchors.length > 0
            ? { boxShadow: `inset 0 -2px ${UI.mark}`, scrollMarginTop: '96px' }
            : {}

          if (segment.kind === 'removed') {
            return (
              <Box
                key={`${segmentIndex}-${partIndex}`}
                component="del"
                {...anchorProps}
                sx={{
                  bgcolor: '#fee2e2',
                  color: '#991b1b',
                  textDecoration: 'line-through',
                  borderRadius: 0.5,
                  ...anchorStyle,
                }}
              >
                {anchorMarkers}
                {text}
              </Box>
            )
          }
          if (segment.kind === 'added') {
            return (
              <Box
                key={`${segmentIndex}-${partIndex}`}
                component="ins"
                {...anchorProps}
                sx={{
                  bgcolor: '#dcfce7',
                  color: '#166534',
                  textDecoration: 'underline',
                  textUnderlineOffset: '2px',
                  borderRadius: 0.5,
                  ...anchorStyle,
                }}
              >
                {anchorMarkers}
                {text}
              </Box>
            )
          }
          return (
            <Box
              key={`${segmentIndex}-${partIndex}`}
              component="span"
              {...anchorProps}
              sx={anchorStyle}
            >
              {anchorMarkers}
              {text}
            </Box>
          )
        })
      })}
    </Box>
  )
})

export default HighlightedDiffText
