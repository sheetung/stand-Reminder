// 2048 Game Logic - Based on gabrielecirulli/2048

class Grid {
  constructor(size, previousState) {
    this.size = size;
    this.cells = previousState ? this.fromState(previousState) : this.empty();
  }

  empty() {
    const cells = [];
    for (let x = 0; x < this.size; x++) {
      cells[x] = [];
      for (let y = 0; y < this.size; y++) {
        cells[x][y] = null;
      }
    }
    return cells;
  }

  fromState(state) {
    const cells = [];
    for (let x = 0; x < this.size; x++) {
      cells[x] = [];
      for (let y = 0; y < this.size; y++) {
        const tile = state[x][y];
        cells[x][y] = tile ? new Tile(tile.position, tile.value) : null;
      }
    }
    return cells;
  }

  randomAvailableCell() {
    const cells = this.availableCells();
    if (cells.length) {
      return cells[Math.floor(Math.random() * cells.length)];
    }
    return null;
  }

  availableCells() {
    const cells = [];
    this.eachCell((x, y, tile) => {
      if (!tile) {
        cells.push({ x: x, y: y });
      }
    });
    return cells;
  }

  cellsAvailable() {
    return this.availableCells().length > 0;
  }

  cellAvailable(cell) {
    return !this.cellOccupied(cell);
  }

  cellOccupied(cell) {
    return !!this.cellContent(cell);
  }

  cellContent(cell) {
    if (this.withinBounds(cell)) {
      return this.cells[cell.x][cell.y];
    }
    return null;
  }

  insertTile(tile) {
    this.cells[tile.x][tile.y] = tile;
  }

  removeTile(tile) {
    this.cells[tile.x][tile.y] = null;
  }

  withinBounds(position) {
    return position.x >= 0 && position.x < this.size &&
           position.y >= 0 && position.y < this.size;
  }

  eachCell(callback) {
    for (let x = 0; x < this.size; x++) {
      for (let y = 0; y < this.size; y++) {
        callback(x, y, this.cells[x][y]);
      }
    }
  }

  serialize() {
    const cellState = [];
    for (let x = 0; x < this.size; x++) {
      cellState[x] = [];
      for (let y = 0; y < this.size; y++) {
        cellState[x][y] = this.cells[x][y] ? this.cells[x][y].serialize() : null;
      }
    }
    return {
      size: this.size,
      cells: cellState
    };
  }
}

class Tile {
  constructor(position, value) {
    this.x = position.x;
    this.y = position.y;
    this.value = value || 2;
    this.previousPosition = null;
    this.mergedFrom = null;
  }

  savePosition() {
    this.previousPosition = { x: this.x, y: this.y };
  }

  updatePosition(position) {
    this.x = position.x;
    this.y = position.y;
  }

  serialize() {
    return {
      position: {
        x: this.x,
        y: this.y
      },
      value: this.value
    };
  }
}

class KeyboardInputManager {
  constructor() {
    this.events = {};
    this.listen();
  }

  on(event, callback) {
    if (!this.events[event]) {
      this.events[event] = [];
    }
    this.events[event].push(callback);
  }

  emit(event, data) {
    const callbacks = this.events[event];
    if (callbacks) {
      callbacks.forEach(callback => callback(data));
    }
  }

  listen() {
    const map = {
      38: 3, // Up -> Left
      39: 2, // Right -> Down
      40: 1, // Down -> Right
      37: 0, // Left -> Up
      75: 3, // Vim up -> Left
      76: 2, // Vim right -> Down
      74: 1, // Vim down -> Right
      72: 0, // Vim left -> Up
      87: 3, // W -> Left
      68: 2, // D -> Down
      83: 1, // S -> Right
      65: 0  // A -> Up
    };

    document.addEventListener('keydown', (event) => {
      const modifiers = event.altKey || event.ctrlKey || event.metaKey || event.shiftKey;
      const mapped = map[event.which];

      if (!modifiers && mapped !== undefined) {
        event.preventDefault();
        this.emit('move', mapped);
      }

      if (!modifiers && event.which === 82) {
        this.emit('restart');
      }
    });

    // Restart button
    const restartButton = document.getElementById('gameNewGame');
    if (restartButton) {
      restartButton.addEventListener('click', () => this.emit('restart'));
    }

    // Close button
    const closeButton = document.getElementById('gameClose');
    if (closeButton) {
      closeButton.addEventListener('click', () => this.emit('close'));
    }

    // Touch events
    let touchStartClientX, touchStartClientY;
    const gameOverlay = document.getElementById('gameOverlay');
    const gameContainer = gameOverlay ? gameOverlay.querySelector('.game-container') : null;

    if (gameContainer) {
      gameContainer.addEventListener('touchstart', (event) => {
        if (event.touches.length > 1 || (!!event.targetTouches && event.targetTouches.length > 1)) {
          return;
        }
        touchStartClientX = event.touches[0].clientX;
        touchStartClientY = event.touches[0].clientY;
        event.preventDefault();
      });

      gameContainer.addEventListener('touchmove', (event) => {
        event.preventDefault();
      });

      gameContainer.addEventListener('touchend', (event) => {
        if (event.touches.length > 0 || (!!event.targetTouches && event.targetTouches.length > 0)) {
          return;
        }

        const touchEndClientX = event.changedTouches[0].clientX;
        const touchEndClientY = event.changedTouches[0].clientY;

        const dx = touchEndClientX - touchStartClientX;
        const dy = touchEndClientY - touchStartClientY;

        const absDx = Math.abs(dx);
        const absDy = Math.abs(dy);

        if (Math.max(absDx, absDy) > 10) {
          const direction = absDx > absDy ? (dx > 0 ? 1 : 3) : (dy > 0 ? 2 : 0);
          this.emit('move', direction);
        }
      });
    }
  }
}

class HTMLActuator {
  constructor() {
    this.tileContainer = document.getElementById('tileContainer');
    this.gridContainer = document.getElementById('gameGrid');
    this.scoreContainer = document.getElementById('gameScore');
    this.messageContainer = null;
    this.score = 0;
    this.createGridCells();
  }

  createGridCells() {
    const self = this;
    self.clearContainer(self.gridContainer);

    for (let y = 0; y < 4; y++) {
      const row = document.createElement('div');
      row.classList.add('grid-row');

      for (let x = 0; x < 4; x++) {
        const cell = document.createElement('div');
        cell.classList.add('grid-cell');
        row.appendChild(cell);
      }

      self.gridContainer.appendChild(row);
    }
  }

  actuate(grid, metadata) {
    const self = this;

    window.requestAnimationFrame(() => {
      // 只清除方块，不清除网格单元格
      self.clearContainer(self.tileContainer);

      grid.cells.forEach((column) => {
        column.forEach((cell) => {
          if (cell) {
            self.addTile(cell);
          }
        });
      });

      self.updateScore(metadata.score);

      if (metadata.terminated) {
        if (metadata.over) {
          self.message(false);
        } else if (metadata.won) {
          self.message(true);
        }
      }
    });
  }

  restart() {
    this.clearMessage();
  }

  clearContainer(container) {
    while (container.firstChild) {
      container.removeChild(container.firstChild);
    }
  }

  addTile(tile) {
    const self = this;

    const wrapper = document.createElement('div');
    const inner = document.createElement('div');
    const position = tile.previousPosition || { x: tile.x, y: tile.y };
    const positionClass = this.positionClass(position);

    const classes = ['tile', 'tile-' + tile.value, positionClass];

    if (tile.value > 2048) classes.push('tile-super');

    this.applyClasses(wrapper, classes);

    inner.classList.add('tile-inner');
    inner.textContent = tile.value;

    if (tile.previousPosition) {
      window.requestAnimationFrame(() => {
        classes[2] = self.positionClass({ x: tile.x, y: tile.y });
        self.applyClasses(wrapper, classes);
      });
    } else if (tile.mergedFrom) {
      classes.push('tile-merged');
      this.applyClasses(wrapper, classes);

      tile.mergedFrom.forEach((merged) => {
        self.addTile(merged);
      });
    } else {
      classes.push('tile-new');
      this.applyClasses(wrapper, classes);
    }

    wrapper.appendChild(inner);
    this.tileContainer.appendChild(wrapper);
  }

  applyClasses(element, classes) {
    element.setAttribute('class', classes.join(' '));
  }

  normalizePosition(position) {
    return { x: position.x + 1, y: position.y + 1 };
  }

  positionClass(position) {
    position = this.normalizePosition(position);
    return 'tile-position-' + position.y + '-' + position.x;
  }

  updateScore(score) {
    this.clearContainer(this.scoreContainer);
    this.score = score;
    this.scoreContainer.textContent = this.score;
  }

  message(won) {
    this.clearMessage();
    const type = won ? 'game-won' : 'game-over';
    const text = won ? 'You win!' : 'Game over!';

    const message = document.createElement('div');
    message.classList.add('game-message', type);
    message.innerHTML = '<p>' + text + '</p><button class="game-button">Try Again</button>';

    this.tileContainer.appendChild(message);

    message.querySelector('button').addEventListener('click', () => {
      this.emit('restart');
    });

    this.messageContainer = message;
  }

  clearMessage() {
    if (this.messageContainer) {
      this.messageContainer.remove();
      this.messageContainer = null;
    }
  }

  emit(event) {
    if (this.events && this.events[event]) {
      this.events[event]();
    }
  }

  on(event, callback) {
    if (!this.events) this.events = {};
    this.events[event] = callback;
  }
}

class LocalStorageManager {
  constructor() {
    this.bestScoreKey = 'bestScore';
    this.gameStateKey = 'gameState';
  }

  getBestScore() {
    const data = localStorage.getItem(this.bestScoreKey);
    return data ? parseInt(data, 10) : 0;
  }

  setBestScore(score) {
    localStorage.setItem(this.bestScoreKey, score);
  }

  getGameState() {
    const data = localStorage.getItem(this.gameStateKey);
    return data ? JSON.parse(data) : null;
  }

  setGameState(gameState) {
    localStorage.setItem(this.gameStateKey, JSON.stringify(gameState));
  }

  clearGameState() {
    localStorage.removeItem(this.gameStateKey);
  }
}

class GameManager {
  constructor(size, InputManager, Actuator, StorageManager) {
    this.size = size;
    this.inputManager = new InputManager();
    this.storageManager = new StorageManager();
    this.actuator = new Actuator();

    this.startTiles = 2;

    this.inputManager.on('move', this.move.bind(this));
    this.inputManager.on('restart', this.restart.bind(this));
    this.inputManager.on('close', this.close.bind(this));

    this.setup();
  }

  restart() {
    this.storageManager.clearGameState();
    this.actuator.restart();
    this.setup();
  }

  keepPlaying() {
    this.keepPlaying = true;
    this.actuator.restart();
  }

  isGameTerminated() {
    return this.over || (this.won && !this.keepPlaying);
  }

  setup() {
    const previousState = this.storageManager.getGameState();

    if (previousState) {
      this.grid = new Grid(previousState.grid.size, previousState.grid.cells);
      this.score = previousState.score;
      this.over = previousState.over;
      this.won = previousState.won;
      this.keepPlaying = previousState.keepPlaying;
    } else {
      this.grid = new Grid(this.size);
      this.score = 0;
      this.over = false;
      this.won = false;
      this.keepPlaying = false;

      this.addStartTiles();
    }

    this.actuate();
  }

  addStartTiles() {
    for (let i = 0; i < this.startTiles; i++) {
      this.addRandomTile();
    }
  }

  addRandomTile() {
    if (this.grid.cellsAvailable()) {
      const value = Math.random() < 0.9 ? 2 : 4;
      const tile = new Tile(this.grid.randomAvailableCell(), value);
      this.grid.insertTile(tile);
    }
  }

  actuate() {
    if (this.storageManager.getBestScore() < this.score) {
      this.storageManager.setBestScore(this.score);
    }

    if (this.over) {
      this.storageManager.clearGameState();
    } else {
      this.storageManager.setGameState(this.serialize());
    }

    this.actuator.actuate(this.grid, {
      score: this.score,
      over: this.over,
      won: this.won,
      bestScore: this.storageManager.getBestScore(),
      terminated: this.isGameTerminated()
    });
  }

  serialize() {
    return {
      grid: this.grid.serialize(),
      score: this.score,
      over: this.over,
      won: this.won,
      keepPlaying: this.keepPlaying
    };
  }

  prepareTiles() {
    this.grid.eachCell((x, y, tile) => {
      if (tile) {
        tile.mergedFrom = null;
        tile.savePosition();
      }
    });
  }

  moveTile(tile, cell) {
    this.grid.cells[tile.x][tile.y] = null;
    this.grid.cells[cell.x][cell.y] = tile;
    tile.updatePosition(cell);
  }

  move(direction) {
    const self = this;

    if (this.isGameTerminated()) return;

    const vector = this.getVector(direction);
    const traversals = this.buildTraversals(vector);
    let moved = false;

    this.prepareTiles();

    traversals.x.forEach((x) => {
      traversals.y.forEach((y) => {
        const cell = { x: x, y: y };
        const tile = self.grid.cellContent(cell);

        if (tile) {
          const positions = self.findFarthestPosition(cell, vector);
          const next = self.grid.cellContent(positions.next);

          if (next && next.value === tile.value && !next.mergedFrom) {
            const merged = new Tile(positions.next, tile.value * 2);
            merged.mergedFrom = [tile, next];

            self.grid.insertTile(merged);
            self.grid.removeTile(tile);

            tile.updatePosition(positions.next);

            self.score += merged.value;

            if (merged.value === 2048) self.won = true;
          } else {
            self.moveTile(tile, positions.farthest);
          }

          if (!self.positionsEqual(cell, tile)) {
            moved = true;
          }
        }
      });
    });

    if (moved) {
      this.addRandomTile();

      if (!this.movesAvailable()) {
        this.over = true;
      }

      this.actuate();
    }
  }

  getVector(direction) {
    const map = {
      0: { x: -1, y: 0 }, // Left (was Up)
      1: { x: 0, y: 1 },  // Down (was Right)
      2: { x: 1, y: 0 },  // Right (was Down)
      3: { x: 0, y: -1 }  // Up (was Left)
    };
    return map[direction];
  }

  buildTraversals(vector) {
    const traversals = { x: [], y: [] };

    for (let pos = 0; pos < this.size; pos++) {
      traversals.x.push(pos);
      traversals.y.push(pos);
    }

    if (vector.x === 1) traversals.x = traversals.x.reverse();
    if (vector.y === 1) traversals.y = traversals.y.reverse();

    return traversals;
  }

  findFarthestPosition(cell, vector) {
    let previous;

    do {
      previous = cell;
      cell = { x: previous.x + vector.x, y: previous.y + vector.y };
    } while (this.grid.withinBounds(cell) && this.grid.cellAvailable(cell));

    return {
      farthest: previous,
      next: cell
    };
  }

  movesAvailable() {
    return this.grid.cellsAvailable() || this.tileMatchesAvailable();
  }

  tileMatchesAvailable() {
    const self = this;

    let tile;

    for (let x = 0; x < this.size; x++) {
      for (let y = 0; y < this.size; y++) {
        tile = this.grid.cellContent({ x: x, y: y });

        if (tile) {
          for (let direction = 0; direction < 4; direction++) {
            const vector = self.getVector(direction);
            const cell = { x: x + vector.x, y: y + vector.y };

            const other = self.grid.cellContent(cell);

            if (other && other.value === tile.value) {
              return true;
            }
          }
        }
      }
    }

    return false;
  }

  positionsEqual(first, second) {
    return first.x === second.x && first.y === second.y;
  }

  close() {
    const overlay = document.getElementById('gameOverlay');
    if (overlay) {
      overlay.style.display = 'none';
    }
  }
}

// Initialize game
document.addEventListener('DOMContentLoaded', () => {
  const breakCard = document.getElementById('breakCard');
  if (breakCard) {
    const gameButton = document.createElement('button');
    gameButton.className = 'quick-secondary';
    gameButton.textContent = 'Play 2048';
    gameButton.style.marginTop = '10px';
    gameButton.style.display = 'block';
    gameButton.style.width = '100%';
    gameButton.addEventListener('click', () => {
      const gameOverlay = document.getElementById('gameOverlay');
      if (gameOverlay) {
        gameOverlay.style.display = 'flex';
      }
    });
    breakCard.appendChild(gameButton);
  }

  // Add click outside to close functionality
  const gameOverlay = document.getElementById('gameOverlay');
  if (gameOverlay) {
    gameOverlay.addEventListener('click', (event) => {
      if (event.target === gameOverlay) {
        const gameManager = window.gameManager;
        if (gameManager && gameManager.close) {
          gameManager.close();
        }
      }
    });
  }

  window.gameManager = new GameManager(4, KeyboardInputManager, HTMLActuator, LocalStorageManager);
});
