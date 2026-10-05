import { Box, Typography } from '@mui/material'
import { LP, RISE, NO_MOTION, delay } from './tokens'

/**
 * 「実際にどう使うか」を一連で見せる節（#1653）。
 *
 * LPで一番伝えたいのは機能の一覧ではなく、学生の一日の中で何が起きるか。
 * 面接で答えに詰まる → 練習する → 何を直せばいいか分かる、までを続きで出す。
 * ここだけ他の節より大きく扱う（情報の強弱）。
 *
 * **数値と文面は表示例。** 実際の出力は学生ごとに変わる。
 * 画面内にも「表示例」と明記すること。実データのように読ませない。
 *
 * 講評の5観点は interview_rubric.go の rubricCriteria と揃える。
 * 「根拠は実際の発言から引く」は実装の挙動（blankUnmatchedEvidence が
 * 発話と照合できなかった根拠を空にする / #1527）。ここを誇張しない。
 */

const STEPS = [
  {
    n: '1',
    when: '面接の前日',
    title: '「学生時代に力を入れたこと」で手が止まる',
    body: '制作課題のことを話そうとしても、何をどう言えばいいか分からない。先生に見てもらう時間も、もう取れない。',
  },
  {
    n: '2',
    when: 'その場で',
    title: '声に出して、何度でも練習する',
    body: 'AI面接官が質問し、声で答える。時間も回数も決まっていないので、言い直しながら固めていける。',
  },
  {
    n: '3',
    when: '終わった直後',
    title: '直すところが、自分の言葉で返ってくる',
    body: '5つの観点で講評が出る。根拠は実際に話した言葉から引用されるので、どこを直すかが分かる。',
  },
] as const

export function LandingWalkthrough() {
  return (
    <Box>
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' },
          borderTop: `2px solid ${LP.ink}`,
          borderLeft: { md: `1px solid ${LP.rule}` },
        }}
      >
        {STEPS.map((s, i) => (
          <Box
            key={s.n}
            sx={{
              p: { xs: 2.5, md: 3 },
              borderRight: { md: `1px solid ${LP.rule}` },
              borderBottom: `1px solid ${LP.rule}`,
              ...RISE,
              ...delay(i),
              ...NO_MOTION,
            }}
          >
            <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 1.5 }}>
              <Typography
                aria-hidden
                sx={{ fontSize: 22, fontWeight: 700, color: LP.seal, lineHeight: 1 }}
              >
                {s.n}
              </Typography>
              <Typography sx={{ fontSize: 11.5, color: LP.muted, letterSpacing: '.08em' }}>
                {s.when}
              </Typography>
            </Box>
            <Typography
              component="h3"
              sx={{ mt: 1.5, fontSize: { xs: 16.5, md: 17.5 }, fontWeight: 700, lineHeight: 1.7 }}
            >
              {s.title}
            </Typography>
            <Typography sx={{ mt: 1.25, fontSize: 13.5, lineHeight: 2.1, color: LP.muted }}>
              {s.body}
            </Typography>
          </Box>
        ))}
      </Box>

      {/* 実際に返ってくる講評の形。ここが一番見せたいので大きく出す。 */}
      <Box
        sx={{
          mt: { xs: 4, md: 5 },
          bgcolor: LP.card,
          border: `1px solid ${LP.rule}`,
          boxShadow: `4px 4px 0 ${LP.ruleSoft}`,
          ...RISE,
          ...delay(3),
          ...NO_MOTION,
        }}
      >
        <Box
          sx={{
            px: { xs: 2.5, md: 3.5 },
            py: 1.75,
            borderBottom: `1px solid ${LP.rule}`,
            display: 'flex',
            alignItems: 'baseline',
            gap: 1.5,
            flexWrap: 'wrap',
          }}
        >
          <Typography sx={{ fontSize: 12.5, fontWeight: 700, letterSpacing: '.1em' }}>
            面接の講評
          </Typography>
          <Typography
            sx={{
              fontSize: 10.5,
              color: LP.muted,
              border: `1px solid ${LP.rule}`,
              px: 0.75,
              py: 0.1,
            }}
          >
            表示例
          </Typography>
        </Box>

        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', md: '1fr 1.25fr' },
            borderBottom: `1px solid ${LP.ruleSoft}`,
          }}
        >
          {/* スコア */}
          <Box
            sx={{
              p: { xs: 2.5, md: 3.5 },
              borderRight: { md: `1px solid ${LP.ruleSoft}` },
              display: 'grid',
              gap: 1.5,
              alignContent: 'start',
            }}
          >
            {[
              { label: '論理性', v: 4 },
              { label: '具体性', v: 2 },
              { label: '主体性', v: 4 },
              { label: 'コミュニケーション力', v: 3 },
              { label: '積極性', v: 4 },
            ].map((s) => (
              <Box
                key={s.label}
                sx={{
                  display: 'grid',
                  gridTemplateColumns: '9.5rem 1fr 1.5rem',
                  alignItems: 'center',
                  gap: 1.25,
                }}
              >
                <Typography sx={{ fontSize: 12, color: LP.muted }}>{s.label}</Typography>
                <Box sx={{ display: 'flex', gap: 0.5 }}>
                  {[1, 2, 3, 4, 5].map((i) => (
                    <Box
                      key={i}
                      sx={{
                        width: 1,
                        flexGrow: 1,
                        height: 8,
                        bgcolor: i <= s.v ? (s.v <= 2 ? LP.seal : LP.primary) : LP.ruleSoft,
                      }}
                    />
                  ))}
                </Box>
                <Typography
                  sx={{
                    fontSize: 12.5,
                    fontWeight: 700,
                    textAlign: 'right',
                    fontVariantNumeric: 'tabular-nums',
                  }}
                >
                  {s.v}
                </Typography>
              </Box>
            ))}
          </Box>

          {/* 指摘と根拠 */}
          <Box sx={{ p: { xs: 2.5, md: 3.5 }, display: 'grid', gap: 2, alignContent: 'start' }}>
            <Box>
              <Typography
                sx={{ fontSize: 11, fontWeight: 700, color: LP.seal, letterSpacing: '.1em' }}
              >
                直すところ
              </Typography>
              <Typography sx={{ mt: 1, fontSize: 14.5, lineHeight: 2, fontWeight: 700 }}>
                「何をしたか」は話せていますが、「どれだけ良くなったか」が入っていません。
              </Typography>
            </Box>
            <Box>
              <Typography
                sx={{ fontSize: 11, fontWeight: 700, color: LP.muted, letterSpacing: '.1em' }}
              >
                根拠（あなたが実際に話した言葉）
              </Typography>
              <Typography
                sx={{
                  mt: 1,
                  fontSize: 13.5,
                  lineHeight: 2.1,
                  color: LP.inkSoft,
                  borderLeft: `2px solid ${LP.rule}`,
                  pl: 1.5,
                }}
              >
                APIの応答が遅くて、原因を切り分けて、キャッシュを入れました。
              </Typography>
              <Typography sx={{ mt: 1.25, fontSize: 12.5, lineHeight: 2, color: LP.muted }}>
                秒数や件数を添えると「具体性」が上がります。
              </Typography>
            </Box>
          </Box>
        </Box>

        <Box sx={{ px: { xs: 2.5, md: 3.5 }, py: 2 }}>
          <Typography sx={{ fontSize: 12, lineHeight: 2, color: LP.muted }}>
            根拠は、実際に話した言葉と照合できたものだけが出ます。照合できない指摘は表示しません。
          </Typography>
        </Box>
      </Box>
    </Box>
  )
}
