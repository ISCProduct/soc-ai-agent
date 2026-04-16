import { Table, TableContainer, type TableContainerProps, type TableProps } from '@mui/material'
import { type ReactNode } from 'react'

type AdminTableWrapperProps = {
  children: ReactNode
  tableProps?: TableProps
  containerProps?: TableContainerProps
}

export function AdminTableWrapper({ children, tableProps, containerProps }: AdminTableWrapperProps) {
  return (
    <TableContainer {...containerProps}>
      <Table size="small" {...tableProps}>
        {children}
      </Table>
    </TableContainer>
  )
}

