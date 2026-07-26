'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { Button, Box } from '@mui/material';
import { Suspense } from 'react';
import dynamic from 'next/dynamic';
import { PageLoading } from '@/components/common/PageLoading';

const CorrelationDiagram = dynamic(() => import('@/components/Correlation-diagram'), {
    ssr: false,
    loading: () => <PageLoading message="企業相関図を準備しています..." />,
});

function CorrelationDiagramContent() {
    const router = useRouter();
    const searchParams = useSearchParams();
    const companyIdParam = searchParams.get('company_id');
    const initialCompanyId = companyIdParam ? parseInt(companyIdParam, 10) : null;

    return (
        <Box sx={{ p: 2 }}>
            <Button variant="contained" onClick={() => router.back()} sx={{ mb: 2 }}>
                戻る
            </Button>

            <CorrelationDiagram initialCompanyId={initialCompanyId} />
        </Box>
    );
}

export default function Page() {
    return (
        <Suspense fallback={<PageLoading message="企業相関図を読み込み中..." />}>
            <CorrelationDiagramContent />
        </Suspense>
    );
}
