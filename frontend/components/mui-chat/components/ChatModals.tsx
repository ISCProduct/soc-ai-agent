'use client'

import {
  Box,
  Button,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Typography,
} from '@mui/material'

type ChatModalsProps = {
  showCompletionModal: boolean
  showEndChatModal: boolean
  showTerminationModal: boolean
  allPhasesCompleted: boolean
  onContinueChat: () => void
  onViewResults: () => void
  onCancelEndChat: () => void
  onConfirmEndChat: () => void
}

/** 完了 / 終了確認 / 強制終了のダイアログ群 */
export function ChatModals({
  showCompletionModal,
  showEndChatModal,
  showTerminationModal,
  allPhasesCompleted,
  onContinueChat,
  onViewResults,
  onCancelEndChat,
  onConfirmEndChat,
}: ChatModalsProps) {
  return (
    <>
      {/* 分析完了モーダル */}
      <Dialog
        open={showCompletionModal}
        onClose={allPhasesCompleted ? undefined : onContinueChat}
        maxWidth="sm"
        fullWidth
        PaperProps={{
          sx: {
            borderRadius: 2,
            p: 2,
          },
        }}
      >
        <DialogTitle sx={{ textAlign: 'center', pb: 1 }}>
          <Typography variant="h5" component="div" sx={{ fontWeight: 'bold', color: 'primary.main' }}>
            🎉 分析が完了しました！
          </Typography>
        </DialogTitle>
        <DialogContent sx={{ pt: 2, pb: 2 }}>
          <Typography variant="body1" sx={{ textAlign: 'center', mb: 2 }}>
            {allPhasesCompleted
              ? 'すべての分析が完了しました！あなたに最適な企業をマッチングしました。'
              : 'あなたの適性を分析し、最適な企業をマッチングしました。'}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ textAlign: 'center' }}>
            結果ページで詳細な企業情報を確認できます。
          </Typography>
        </DialogContent>
        <DialogActions sx={{ justifyContent: 'center', gap: 2, pb: 2 }}>
          {!allPhasesCompleted && (
            <Button
              onClick={onContinueChat}
              variant="outlined"
              size="large"
              sx={{ minWidth: 140 }}
            >
              チャットを続ける
            </Button>
          )}
          <Button
            onClick={onViewResults}
            variant="contained"
            size="large"
            sx={{ minWidth: 140 }}
          >
            結果を見る
          </Button>
        </DialogActions>
      </Dialog>

      {/* チャット終了確認モーダル */}
      <Dialog
        open={showEndChatModal}
        onClose={onCancelEndChat}
        maxWidth="sm"
        fullWidth
        PaperProps={{
          sx: {
            borderRadius: 2,
            p: 2,
          },
        }}
      >
        <DialogTitle sx={{ textAlign: 'center', pb: 1 }}>
          <Typography variant="h5" component="div" sx={{ fontWeight: 'bold', color: 'warning.main' }}>
            ⚠️ チャットを終了しますか？
          </Typography>
        </DialogTitle>
        <DialogContent sx={{ pt: 2, pb: 2 }}>
          <Typography variant="body1" sx={{ textAlign: 'center', mb: 2 }}>
            チャットを終了すると、現在の会話履歴が削除されます。
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ textAlign: 'center' }}>
            新しいセッションで最初からやり直すことになりますが、よろしいですか？
          </Typography>
        </DialogContent>
        <DialogActions sx={{ justifyContent: 'center', gap: 2, pb: 2 }}>
          <Button
            onClick={onCancelEndChat}
            variant="outlined"
            size="large"
            sx={{ minWidth: 140 }}
          >
            キャンセル
          </Button>
          <Button
            onClick={onConfirmEndChat}
            variant="contained"
            color="error"
            size="large"
            sx={{ minWidth: 140 }}
          >
            終了する
          </Button>
        </DialogActions>
      </Dialog>

      {/* 強制終了モーダル（3回の無効回答） */}
      <Dialog
        open={showTerminationModal}
        onClose={() => {}} // 閉じられないようにする
        disableEscapeKeyDown // Escキーでも閉じられない
        maxWidth="sm"
        fullWidth
        PaperProps={{
          sx: {
            borderRadius: 2,
            p: 2,
          },
        }}
      >
        <DialogTitle sx={{ textAlign: 'center', pb: 1 }}>
          <Typography variant="h5" component="div" sx={{ fontWeight: 'bold', color: 'error.main' }}>
            ⚠️ チャットを終了します
          </Typography>
        </DialogTitle>
        <DialogContent sx={{ pt: 2, pb: 2 }}>
          <Typography
            variant="body1"
            sx={{ textAlign: 'center', mb: 2, color: 'error.main', fontWeight: 'bold' }}
          >
            質問と関係のない内容が3回続いたため、チャットを終了しました。
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ textAlign: 'center', mb: 1 }}>
            新しいセッションで最初からやり直してください。
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ textAlign: 'center' }}>
            現在の回答内容は保存されていません。
          </Typography>
        </DialogContent>
        <DialogActions sx={{ justifyContent: 'center', pb: 2 }}>
          <Button
            onClick={() => {
              // 新しいセッションを開始（ページリロード）
              sessionStorage.removeItem('chatSessionId')
              localStorage.removeItem('currentSessionId')
              window.location.reload()
            }}
            variant="contained"
            color="error"
            size="large"
            sx={{ minWidth: 180 }}
            autoFocus={false}
            tabIndex={-1}
          >
            新しいセッションを開始
          </Button>
        </DialogActions>
      </Dialog>
    </>
  )
}
