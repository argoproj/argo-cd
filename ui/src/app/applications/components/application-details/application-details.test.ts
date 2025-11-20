import { Application } from '../../../shared/models';
import {GetBreadcrumbs} from './application-details';

const APP_LABEL_KEY = 'app.kubernetes.io/instance';

test('getBreadcrumbs.root', () => {
    const breadcrumbs = GetBreadcrumbs({metadata: {name: 'root'}} as Application, APP_LABEL_KEY);

    expect(breadcrumbs).toHaveLength(2);
    expect(breadcrumbs[0]).toStrictEqual({"path": "/applications", "title": "Applications"});
});

test('getBreadcrumbs.appLabelKeyEmpty', () => {
    const breadcrumbs = GetBreadcrumbs({metadata: {name: 'root'}} as Application, '');

    expect(breadcrumbs).toHaveLength(2);
    expect(breadcrumbs[0]).toStrictEqual({"path": "/applications", "title": "Applications"});
});

test('getBreadcrumbs.child', () => {
    const breadcrumbs = GetBreadcrumbs({
        metadata: { name: 'child', labels: {[APP_LABEL_KEY]: 'root'} }
    } as unknown as Application, APP_LABEL_KEY);

    expect(breadcrumbs).toHaveLength(3);
    expect(breadcrumbs[0]).toStrictEqual({"path": "/applications", "title": "Applications"});
    expect(breadcrumbs[1]).toStrictEqual({"path": "/applications/root", "title": "root"});
});