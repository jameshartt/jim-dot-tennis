import { CDPSession, Page } from "@playwright/test";

/**
 * A Chromium virtual authenticator (CDP WebAuthn domain). Chromium-only.
 *
 * It behaves like a phone or laptop that keeps discoverable credentials,
 * verifies the user and approves every prompt automatically, so
 * navigator.credentials.create()/get() succeed without a human. WebAuthn needs
 * a secure context: these specs run in the "auth" project, where the browser
 * reaches the app as http://localhost:8080.
 *
 * An authenticator belongs to one page. Add it before the click that triggers
 * the ceremony.
 */
export class VirtualAuthenticator {
  private constructor(
    private readonly cdp: CDPSession,
    private authenticatorId: string,
  ) {}

  static async add(page: Page): Promise<VirtualAuthenticator> {
    const cdp = await page.context().newCDPSession(page);
    await cdp.send("WebAuthn.enable");
    const { authenticatorId } = await cdp.send("WebAuthn.addVirtualAuthenticator", {
      options: {
        protocol: "ctap2",
        transport: "internal",
        hasResidentKey: true,
        hasUserVerification: true,
        isUserVerified: true,
        automaticPresenceSimulation: true,
      },
    });
    return new VirtualAuthenticator(cdp, authenticatorId);
  }

  /**
   * An authenticator holding the given credentials (as `credentials()`
   * returned them): the same device brought back after `remove()`.
   */
  static async restore(page: Page, credentials: unknown[]): Promise<VirtualAuthenticator> {
    const authenticator = await VirtualAuthenticator.add(page);
    for (const credential of credentials) {
      await authenticator.cdp.send("WebAuthn.addCredential", {
        authenticatorId: authenticator.authenticatorId,
        credential: credential as never,
      });
    }
    return authenticator;
  }

  /** The credentials this authenticator holds, to bring it back with `restore()`. */
  async credentials(): Promise<unknown[]> {
    const { credentials } = await this.cdp.send("WebAuthn.getCredentials", {
      authenticatorId: this.authenticatorId,
    });
    return credentials;
  }

  /**
   * Put the device away: forget the authenticator but keep what it held, so
   * `restore()` can bring it back. Only one device at a time is attached to a
   * page here, as only one would be in someone's hand.
   */
  async putAway(): Promise<unknown[]> {
    const held = await this.credentials();
    await this.remove();
    return held;
  }

  /** Number of credentials this authenticator currently holds. */
  async credentialCount(): Promise<number> {
    const { credentials } = await this.cdp.send("WebAuthn.getCredentials", {
      authenticatorId: this.authenticatorId,
    });
    return credentials.length;
  }

  /**
   * Forget the authenticator and every credential on it. The server still
   * knows the credentials; use this to get a clean device so a second passkey
   * can be created (the server asks authenticators holding an existing
   * credential to refuse).
   */
  async remove(): Promise<void> {
    await this.cdp.send("WebAuthn.removeVirtualAuthenticator", {
      authenticatorId: this.authenticatorId,
    });
  }
}
