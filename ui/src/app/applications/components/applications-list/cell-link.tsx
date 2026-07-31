import * as React from 'react';

// Wraps a list-row cell's content in an <a> so middle-click / right-click / status-bar URL
// preview work on the cell itself. tabIndex={-1} keeps it out of the keyboard tab order — the
// row's overlay anchor is the single tab stop carrying the row's link semantics. The cell
// content remains in the a11y tree so screen readers can still read it; the trade-off is that
// SR link lists will show one entry per CellLink (same href as the overlay), which is the
// accepted cost for preserving mouse affordances on cell content. Defined at module scope so
// children (e.g. <Cluster>) don't remount on each parent re-render.
//
// Used by both ApplicationTableRow and AppSetTableRow, so it lives here rather than inline.
export const CellLink = ({
    href,
    onClick,
    className,
    children
}: {
    href: string;
    onClick: (e: React.MouseEvent<HTMLAnchorElement>) => void;
    className?: string;
    children: React.ReactNode;
}) => (
    <a className={`applications-list__table-row__cell-link${className ? ` ${className}` : ''}`} href={href} onClick={onClick} tabIndex={-1}>
        {children}
    </a>
);
