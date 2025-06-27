import { Container, getContainer } from '@cloudflare/containers';

export class ThumbgenContainer extends Container {
  // Configure default port for the container
  defaultPort = 4499;
  sleepAfter = "5m";

  override onStart() {
    console.log('Thumbgen container successfully started ...');
  }

  override onStop() {
    console.log('Thumbgen container successfully shut down ...');
  }

  override onError(error: unknown) {
    console.log('Thumbgen container error: ...', error);
  }
}

export default {
  async fetch(request, env) {
    return getContainer(env.THUMBGEN_CONTAINER).fetch(request);
  },
};