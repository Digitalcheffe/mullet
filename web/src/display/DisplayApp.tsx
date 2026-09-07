import { useParams } from 'react-router-dom';

// Display renderer entry point. The grid engine, screen rotation, and
// top/bottom bars are built out in issue #23.
export default function DisplayApp() {
  const { slug } = useParams<{ slug: string }>();

  return (
    <div className="display-app">
      <p>Display scaffold for "{slug}" — grid renderer added in a later issue.</p>
    </div>
  );
}
