describe('Global Multi-Application Diff View E2E', () => {
    beforeEach(() => {
        // Visit the applications list page
        cy.visit('/applications');
    });

    it('should display the "View Diffs" toolbar button, open the modal, filter/expand accordions, and trigger sync', () => {
        // 1. Verify "View Diffs" button is visible in the toolbar
        cy.contains('button', 'View Diffs')
            .should('be.visible')
            .within(() => {
                // OutOfSync count badge should be displayed
                cy.get('.badge').should('exist');
            });

        // 2. Open the Global Diff Modal
        cy.contains('button', 'View Diffs').click();
        cy.get('.sliding-panel').should('be.visible');

        // 3. Verify total out-of-sync applications stats in the header
        cy.contains('Out-of-Sync Apps:').should('be.visible');
        cy.contains('Drifted Resources:').should('be.visible');

        // 4. Verify quick action buttons exist
        cy.contains('button', 'Sync All Selected').should('be.visible');
        cy.contains('button', 'Refresh').should('be.visible');

        // 5. Test search filter input
        cy.get('input[placeholder="Filter by Kind, Namespace, or Application Name..."]').should('be.visible').type('app-1');

        // Verify filtered results show matching accordion
        cy.get('.application-diff-accordion').should('contain', 'app-1');

        // 6. Expand the accordion to show details
        cy.get('.application-diff-accordion').contains('.application-diff-accordion__name', 'app-1').click();

        // 7. Verify sync button on the accordion header works
        cy.get('.application-diff-accordion').contains('button', 'Sync').should('be.visible').click();
    });
});
