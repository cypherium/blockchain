/*
 * Copyright (C) 2018 The Cypherium Blockchain authors
 *
 * This file is part of the Cypherium Blockchain library.
 *
 * The Cypherium Blockchain library is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The Cypherium Blockchain library is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with the Cypherium Blockchain library. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package blockchain

const (
	AddrLen   = 20
	PubKeyLen = 32
	PrvKeyLen = 64
	HashLen   = 32
	SigLen    = 64
	AggrLen   = 64
)

type (
	Address   [AddrLen]byte
	PubKey    [PubKeyLen]byte
	PrvKey    [PrvKeyLen]byte
	Hash      [HashLen]byte
	Signature [SigLen]byte
	AggrAddr  [AggrLen]byte
)
