import {PageContext} from 'argo-ui';
import * as React from 'react';
import Helmet from 'react-helmet';
import {Link} from 'react-router-dom';

require('./page.scss');

interface PageProps extends React.PropsWithChildren<any> {
    title: string;
    breadcrumbs?: {title: string; path?: string}[];
}

export const NewPage = (props: PageProps) => {
    const {title, breadcrumbs} = props;
    return (
        <div>
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
            <div className='new-toolbar'>
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
                            nodes.push(<span key={`${breadcrumb.title}_sep`} />);
                        }
                        return nodes;
                    })}
                </div>
                <div style={{marginLeft: 'auto'}}>{props.title?.toUpperCase()}</div>
            </div>
            {props.children}
        </div>
    );
};
