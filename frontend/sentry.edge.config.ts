import * as Sentry from '@sentry/nextjs'
import { sentrySharedOptions } from './lib/sentry'

const opts = sentrySharedOptions()
if (opts) {
  Sentry.init(opts)
}
