import * as React from 'react';

// Shared definition-list primitive for the app/appset tile and table views.
//
// Both views are the same thing: lists of `label: value` pairs for one entry. Each pair is a
// <dt>/<dd>; a group of pairs is one <dl> (`EntryFieldList`). The tile is a single full-width
// <dl>; the table is a flex row of several <dl> columns (plus the favourite + actions chrome),
// so each column sizes independently — no cross-column height coupling.
//
// To avoid re-deriving styling the list views already have, each <dt>/<dd> reuses the existing
// utility classes — the tile field classes and the table meta classes — so spacing and truncation
// match the legacy markup. Table labels are shown only at xxlarge and hidden below it via an
// sr-only treatment in entry-fields.scss (see the note on FIELD_CLASSES below). The only new CSS
// is the flex row + the <dl> 2-column grid.

export type EntryVariant = 'tile' | 'table';

const VariantContext = React.createContext<EntryVariant>('tile');

// Reused, already-tuned cell classes per variant. In the table the label is shown only at
// xxlarge, but (unlike the legacy `show-for-xxlarge`, which is display:none and would drop the
// field name from the a11y tree on narrow screens) entry-fields.scss hides it with an sr-only
// treatment below xxlarge, so screen readers still announce the field name.
const FIELD_CLASSES: Record<EntryVariant, {dt: string; dd: string}> = {
    tile: {
        dt: 'applications-tiles__field-label',
        dd: 'applications-tiles__field-value'
    },
    table: {
        dt: 'applications-list__meta-label',
        dd: 'applications-list__meta-value'
    }
};

export interface EntryFieldProps {
    name: string;
    label: string;
    children: React.ReactNode;
    // Extra class on the <dd> (e.g. `applications-table-source` for the source/labels split).
    valueClassName?: string;
}

export const EntryField = ({name, label, children, valueClassName}: EntryFieldProps) => {
    const cls = FIELD_CLASSES[React.useContext(VariantContext)];
    return (
        <>
            <dt className={`entry-fields__dt entry-fields__dt--${name} ${cls.dt}`} title={`${label}:`}>
                {label}:
            </dt>
            <dd className={`entry-fields__dd entry-fields__dd--${name} ${cls.dd}${valueClassName ? ` ${valueClassName}` : ''}`}>{children}</dd>
        </>
    );
};

// One column / group of pairs, rendered as a <dl> that is a full-width 2-column grid.
export const EntryFieldList = ({variant, className, children}: {variant: EntryVariant; className?: string; children: React.ReactNode}) => (
    <VariantContext.Provider value={variant}>
        <dl className={`entry-fields entry-fields--${variant}${className ? ` ${className}` : ''}`}>{children}</dl>
    </VariantContext.Provider>
);
