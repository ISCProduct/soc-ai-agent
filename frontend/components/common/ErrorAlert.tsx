import { Alert, type AlertProps } from '@mui/material'

type ErrorAlertProps = Omit<AlertProps, 'severity' | 'children'> & {
  error?: string | null
}

export function ErrorAlert({ error, sx, ...rest }: ErrorAlertProps) {
  if (!error) return null
  const sxList = Array.isArray(sx) ? sx : [sx]
  return (
    <Alert severity="error" {...rest} sx={[{ mb: 2 }, ...sxList]}>
      {error}
    </Alert>
  )
}

