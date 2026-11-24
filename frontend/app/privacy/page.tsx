import { Box, Container, Typography, Divider } from '@mui/material'
import { BackButton } from '@/components/common/BackButton'

export default function PrivacyPage() {
  return (
    <Container maxWidth="md" sx={{ py: 6 }}>
      <BackButton />

      <Typography variant="h4" component="h1" fontWeight="bold" gutterBottom>
        プライバシーポリシー
      </Typography>
      <Typography variant="body2" color="text.secondary" gutterBottom>
        最終更新日: 2026年11月19日
      </Typography>
      <Divider sx={{ my: 3 }} />

      <Box sx={{ '& h2': { mt: 4, mb: 1 }, '& p': { mb: 2 }, lineHeight: 1.8 }}>
        <Typography variant="h6" component="h2" fontWeight="bold">
          1. 収集する情報
        </Typography>
        <Typography variant="body1">当サービスは、以下の個人情報を収集します。</Typography>
        <Typography variant="body1" component="ul" sx={{ pl: 3 }}>
          <li>氏名・メールアドレス（アカウント登録時）</li>
          <li>チャットの会話内容（就職軸の分析に使用）</li>
          <li>職務経歴書・履歴書（アップロードされた場合）</li>
          <li>面接練習の録画動画・音声（セッション中に収録）</li>
          <li>企業マッチングスコア（算出された分析結果）</li>
        </Typography>

        <Typography variant="h6" component="h2" fontWeight="bold">
          2. 情報の利用目的
        </Typography>
        <Typography variant="body1">収集した情報は以下の目的にのみ使用します。</Typography>
        <Typography variant="body1" component="ul" sx={{ pl: 3 }}>
          <li>就職軸の分析・企業マッチング機能の提供</li>
          <li>面接練習のフィードバック生成</li>
          <li>同意いただいた場合に限り、面接の記録（発話・スコア・録画した動画）を面接の採点モデルの学習に使用（プロフィールの設定でいつでも変更できます）</li>
          <li>サービスの改善・不具合対応</li>
          <li>利用状況の集計（個人を特定しない統計情報として）</li>
        </Typography>

        <Typography variant="h6" component="h2" fontWeight="bold">
          3. 第三者への提供
        </Typography>
        <Typography variant="body1">
          収集した個人情報を、以下の場合を除き第三者に提供しません。
        </Typography>
        <Typography variant="body1" component="ul" sx={{ pl: 3 }}>
          <li>法令に基づく場合</li>
          <li>ユーザー本人の同意がある場合</li>
          <li>機能の提供のため、生成AI・音声認識・音声合成を行う外部の事業者へ、処理を委託する場合（下記）</li>
        </Typography>
        <Typography variant="body1">
          チャット・面接・ES・職務経歴書の文章は、氏名・メールアドレス・電話番号・住所・生年月日などの個人を特定できる情報を、送信前に伏字にしたうえで、外部のAI事業者へ送って処理します。伏字にできない場合は送信しません。
          面接の音声は、文字起こしのために外部のAI事業者へ送信します。音声は伏字にできないため、面接中にお名前や連絡先を話された場合、その内容が送信先に渡ります。
        </Typography>
        <Typography variant="body1" color="error.main" fontWeight="bold">
          ※ 面接動画・職務経歴書は企業に提供しません。当サービス内での分析にのみ使用します。
        </Typography>

        <Typography variant="h6" component="h2" fontWeight="bold">
          4. データの保持期間
        </Typography>
        <Typography variant="body1">
          アカウントが有効な期間中、データを保持します。退会後は即座にログイン・閲覧ができなくなり、12ヶ月後にすべてのデータを完全削除します。
          面接動画は、アカウントが有効な間保管し、退会後12ヶ月で削除します。
        </Typography>

        <Typography variant="h6" component="h2" fontWeight="bold">
          5. ユーザーの権利（個人情報保護法第28条）
        </Typography>
        <Typography variant="body1">ユーザーはいつでも以下の権利を行使できます。</Typography>
        <Typography variant="body1" component="ul" sx={{ pl: 3 }}>
          <li>保有する個人データの開示請求</li>
          <li>個人データの訂正・追加・削除請求</li>
          <li>アカウントの退会（プロフィールページから実行可能。保持期間後に完全削除）</li>
        </Typography>

        <Typography variant="h6" component="h2" fontWeight="bold">
          6. セキュリティ
        </Typography>
        <Typography variant="body1">
          個人情報は暗号化して保存・転送します。パスワードはハッシュ化して保管し、平文では保存しません。
        </Typography>

        <Typography variant="h6" component="h2" fontWeight="bold">
          7. お問い合わせ
        </Typography>
        <Typography variant="body1">
          プライバシーに関するお問い合わせは、サービス内のお問い合わせフォームよりご連絡ください。
        </Typography>
      </Box>
    </Container>
  )
}
