'use client'

'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { Button, Box } from '@mui/material';
import { Suspense } from 'react';
import dynamic from 'next/dynamic';
import { PageLoading } from '@/components/common/PageLoading';
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer';

const CorrelationDiagram = dynamic(() => import('@/components/Correlation-diagram'), {
    ssr: false,
    loading: () => <PageLoading message="企業相関図を準備しています..." />,
});

function CorrelationDiagramContent() {
    const router = useRouter();
    const searchParams = useSearchParams();
    const companyIdParam = searchParams.get('company_id');
    const parsed = companyIdParam ? parseInt(companyIdParam, 10) : NaN;
    const initialCompanyId = Number.isInteger(parsed) && parsed > 0 ? parsed : null;

    return (
        <Box
            sx={{
                height: '100dvh',
                maxHeight: '100dvh',
                display: 'flex',
                flexDirection: 'column',
                overflow: 'hidden',
            }}
        >
            <Box sx={{ px: 2, pt: 1.5, pb: 1, flexShrink: 0 }}>
                <Button variant="contained" onClick={() => router.back()}>
                    戻る
                </Button>
            </Box>

            <Box sx={{ flex: 1, minHeight: 0, px: { xs: 0, sm: 2 }, pb: { xs: 0, sm: 2 } }}>
                <Box
                    sx={{
                        height: '100%',
                        border: { xs: 'none', sm: '1px solid #e0e0e0' },
                        borderRadius: { xs: 0, sm: 1 },
                        overflow: 'hidden',
                        bgcolor: '#fff',
                    }}
                >
                    <CorrelationDiagram initialCompanyId={initialCompanyId} />
                </Box>
            </Box>
            <BottomNavSpacer />
        </Box>
    );
}

export default function PageContent() {
    return (
        <Suspense
            fallback={
                <Box sx={{ height: '100dvh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    読み込み中...
                </Box>
            }
        >
            <CorrelationDiagramContent />
        </Suspense>
    );
}
