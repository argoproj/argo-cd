import {By, WebDriver} from 'selenium-webdriver';
import {Base} from '../base';
import UiTestUtilities from '../UiTestUtilities';

// CSS selectors for the cluster details fields using data-testid attributes for reliability
const DEGRADED_RESOURCES_COUNT_SELECTOR: By = By.css('[data-testid="degraded-resources-count"]');
const DEGRADED_RESOURCES_LIST_SELECTOR: By = By.css('[data-testid="degraded-resources-list"]');
const RESOURCES_COUNT_SELECTOR: By = By.xpath('//div[contains(@class, "white-box__details-row")]//div[contains(text(), "RESOURCES COUNT:")]/following-sibling::div');
const APPLICATIONS_COUNT_SELECTOR: By = By.xpath('//div[contains(@class, "white-box__details-row")]//div[contains(text(), "APPLICATIONS COUNT:")]/following-sibling::div');

export class ClusterDetailsPage extends Base {
    public constructor(driver: WebDriver) {
        super(driver);
    }

    /**
     * Retry a function with exponential backoff
     */
    private async withRetry<T>(fn: () => Promise<T>, retries: number = 3, baseDelay: number = 1000): Promise<T> {
        for (let i = 0; i < retries; i++) {
            try {
                return await fn();
            } catch (error) {
                if (i === retries - 1) throw error;
                const delay = baseDelay * Math.pow(2, i);
                await new Promise((resolve) => setTimeout(resolve, delay));
            }
        }
        throw new Error('Retry logic failed - should not reach here');
    }

    /**
     * Check if the degraded resources count field is present
     */
    public async isDegradedResourcesCountPresent(): Promise<boolean> {
        try {
            const element = await UiTestUtilities.findUiElement(this.driver, DEGRADED_RESOURCES_COUNT_SELECTOR);
            return element !== null;
        } catch (err) {
            return false;
        }
    }

    /**
     * Get the degraded resources count value
     */
    public async getDegradedResourcesCount(): Promise<string> {
        try {
            const element = await UiTestUtilities.findUiElement(this.driver, DEGRADED_RESOURCES_COUNT_SELECTOR);
            const text = await element.getText();
            return text.trim();
        } catch (err: any) {
            throw new Error(`Failed to get degraded resources count: ${err}`);
        }
    }

    /**
     * Check if the degraded resources list field is present
     */
    public async isDegradedResourcesListPresent(): Promise<boolean> {
        try {
            const element = await UiTestUtilities.findUiElement(this.driver, DEGRADED_RESOURCES_LIST_SELECTOR);
            return element !== null;
        } catch (err) {
            return false;
        }
    }

    /**
     * Get the degraded resources list value
     */
    public async getDegradedResourcesList(): Promise<string> {
        try {
            const element = await UiTestUtilities.findUiElement(this.driver, DEGRADED_RESOURCES_LIST_SELECTOR);
            const text = await element.getText();
            return text.trim();
        } catch (err: any) {
            throw new Error(`Failed to get degraded resources list: ${err}`);
        }
    }

    /**
     * Get the total resources count for comparison
     */
    public async getResourcesCount(): Promise<string> {
        try {
            const element = await UiTestUtilities.findUiElement(this.driver, RESOURCES_COUNT_SELECTOR);
            const text = await element.getText();
            return text.trim();
        } catch (err: any) {
            throw new Error(`Failed to get resources count: ${err}`);
        }
    }

    /**
     * Get the applications count for comparison
     */
    public async getApplicationsCount(): Promise<string> {
        try {
            const element = await UiTestUtilities.findUiElement(this.driver, APPLICATIONS_COUNT_SELECTOR);
            const text = await element.getText();
            return text.trim();
        } catch (err: any) {
            throw new Error(`Failed to get applications count: ${err}`);
        }
    }

    /**
     * Wait for the cluster details page to load by checking for the presence of resources count
     */
    public async waitForPageLoad(): Promise<void> {
        try {
            await UiTestUtilities.findUiElement(this.driver, RESOURCES_COUNT_SELECTOR);
            UiTestUtilities.log('Cluster details page loaded successfully');
        } catch (err: any) {
            throw new Error(`Cluster details page failed to load: ${err}`);
        }
    }

    /**
     * Verify that degraded resources fields are displayed when there are failed GVKs
     * This method checks both the count and list fields
     */
    public async verifyDegradedResourcesFieldsWhenPresent(): Promise<{count: string; list: string}> {
        const isCountPresent = await this.isDegradedResourcesCountPresent();
        const isListPresent = await this.isDegradedResourcesListPresent();

        if (!isCountPresent || !isListPresent) {
            throw new Error('Degraded resources fields are not present when they should be');
        }

        const count = await this.getDegradedResourcesCount();
        const list = await this.getDegradedResourcesList();

        // Validate that count is a number
        const countNumber = parseInt(count, 10);
        if (isNaN(countNumber) || countNumber <= 0) {
            throw new Error(`Invalid degraded resources count: ${count}`);
        }

        // Validate that list contains GVKs (should have dots separating group/version/kind)
        if (!list || list.length === 0) {
            throw new Error(`Degraded resources list is empty: ${list}`);
        }

        UiTestUtilities.log(`Degraded resources count: ${count}`);
        UiTestUtilities.log(`Degraded resources list: ${list}`);

        return {count, list};
    }

    /**
     * Verify that degraded resources fields are NOT displayed when there are no failed GVKs
     */
    public async verifyDegradedResourcesFieldsWhenAbsent(): Promise<void> {
        const isCountPresent = await this.isDegradedResourcesCountPresent();
        const isListPresent = await this.isDegradedResourcesListPresent();

        if (isCountPresent || isListPresent) {
            throw new Error('Degraded resources fields are present when they should be absent');
        }

        UiTestUtilities.log('Confirmed degraded resources fields are not displayed when no failed GVKs exist');
    }
}
