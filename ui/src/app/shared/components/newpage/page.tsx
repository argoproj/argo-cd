import {PageContext} from 'argo-ui';
import {ActionButton, ActionButtonProps} from 'argo-ui/v2';
import * as React from 'react';
import Helmet from 'react-helmet';
import {Link} from 'react-router-dom';

require('./page.scss');

interface PageProps extends React.PropsWithChildren<any> {
    title: string;
    breadcrumbs?: {title: string; path?: string}[];
    actions?: ActionButtonProps[];
    views?: {icon: string; action: () => void; selected?: boolean}[];
}

export const NewPage = (props: PageProps) => {
    const {title, breadcrumbs} = props;
    return (
        <div style={{flex: '1'}}>
            <PageContext.Consumer>
                {ctx => {
                    let titleParts = [ctx.title];
                    if (breadcrumbs && breadcrumbs.length > 0) {
                        titleParts = [
                            breadcrumbs
                                .map(item => item.title)
                                .reverse()
                                .join(' / ')
                        ].concat(titleParts);
                    } else if (title) {
                        titleParts = [title].concat(titleParts);
                    }
                    return (
                        <Helmet>
                            <title>{titleParts.join(' - ')}</title>
                        </Helmet>
                    );
                }}
            </PageContext.Consumer>
            <div className='toolbar toolbar--breadcrumbs'>
                <div style={{marginRight: 'auto'}}>
                    {(breadcrumbs || []).map((breadcrumb, i) => {
                        const nodes = [];
                        if (i === breadcrumbs.length - 1) {
                            nodes.push(<span key={breadcrumb.title}>{breadcrumb.title}</span>);
                        } else {
                            nodes.push(
                                <Link key={breadcrumb.title} to={breadcrumb.path}>
                                    {' '}
                                    {breadcrumb.title}{' '}
                                </Link>
                            );
                        }
                        if (i < breadcrumbs.length - 1) {
                            nodes.push(<span key={`${breadcrumb.title}_sep`}>/ </span>);
                        }
                        return nodes;
                    })}
                </div>
                <div style={{marginLeft: 'auto'}}>{props.title?.toUpperCase()}</div>
            </div>
            {props.actions && (
                <div className='toolbar toolbar--actions'>
                    {props.actions.map(a => (
                        <ActionButton key={a.label} {...a} />
                    ))}
                    {props.views && (
                        <div className='toolbar--actions__views'>
                            {props.views.map(v => (
                                <i className={`fas ${v.icon} toolbar--actions__view ${v.selected ? 'toolbar--actions__view--selected' : ''}`} onClick={v.action} />
                            ))}
                        </div>
                    )}
                </div>
            )}

            {props.children}
        </div>
    );
};
