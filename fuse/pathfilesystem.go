package fuse

import (
	"bytes"
	"sync"
	"fmt"
	"path"
	"strings"
)

type inodeData struct {
	Parent *inodeData
	NodeId uint64
	Name   string
	Count  int
}


// Should implement some hash table method instead? 
func inodeDataKey(parentInode uint64, name string) string {
	// TODO - use something more efficient than Sprintf.
	return fmt.Sprintf("%x:%s", parentInode, name)
}

func (self *inodeData) Key() string {
	var p uint64 = 0
	if self.Parent != nil {
		p = self.Parent.NodeId
	}
	return inodeDataKey(p, self.Name)
}

func (self *inodeData) GetPath() string {
	// TODO - softcode this.
	var components [100]string

	j := len(components)
	for p := self; p != nil && p.NodeId != FUSE_ROOT_ID; p = p.Parent {
		j--
		components[j] = p.Name
	}

	fullPath := strings.Join(components[j:], "/")
	return fullPath
}

type PathFileSystemConnector struct {
	fileSystem PathFilesystem

	// Protects the hashmap, its contents and the nextFreeInode counter.
	lock                sync.RWMutex
	inodePathMap        map[string]*inodeData
	inodePathMapByInode map[uint64]*inodeData
	nextFreeInode uint64
}

func (self *PathFileSystemConnector) lookupUpdate(nodeId uint64, name string) *inodeData {
	self.lock.Lock()
	defer self.lock.Unlock()

	key := inodeDataKey(nodeId, name)
	data, ok := self.inodePathMap[key]
	if !ok {
		data = new(inodeData)
		data.Parent = parent
		data.NodeId = self.nextFreeInode
		data.Name = name
		data.Count = 0

		self.nextFreeInode++

		self.inodePathMapByInode[data.NodeId] = data
		self.inodePathMap[key] = data
	}

	data.Count++
	return data
}

func (self *PathFileSystemConnector) getInodeData(nodeid uint64) *inodeData {
	self.lock.RLock()
	defer self.lock.RUnlock()

	return self.inodePathMapByInode[nodeid]
}

func (self *PathFileSystemConnector) forgetUpdate(nodeId uint64, forgetCount int) {
	self.lock.Lock()
	defer self.lock.Unlock()

	data, ok := self.inodePathMapByInode[nodeId]
	if ok {
		data.Count -= forgetCount
		if data.Count <= 0 {
			self.inodePathMap[data.Key()] = nil, false
			self.inodePathMapByInode[h.NodeId] = nil, false
		}
	}
}

func (self *PathFileSystemConnector) renameUpdate(oldNode uint64,  oldName string, newNode uint64, newName string) {	
 	self.lock.Lock()
	defer self.lock.Unlock()

	oldKey = inodeDataKey(oldNode, oldName)
	data := self.inodePathMap[oldKey]
	if data == nil {
		// This can happen if a rename raced with an unlink or
		// another rename.
		//
		// TODO - does the VFS layer allow this?
		//
		// TODO - is this an error we should signal? 
		return
	}

	self.inodePathMap[oldKey] = nil, false
	
	data.Parent = self.inodePathMapByInode[input.Newdir]
	data.Name = newName

	if data.Parent == nil {
		panic("Moved to unknown node.")
	}
	
	target := self.inodePathMap[data.Key()]
	if target != nil {
		// This could happen if some other thread creates a
		// file in the destination position.
		//
		// TODO - Does the VFS layer allow this?
		//
		// fuse.c just removes the node from its internal
		// tables, which will break things if it is already
		// referenced as a parent object.
		self.inodePathMap[newKey] = nil, false

		target.Parent = self.inodePathMapByInode[FUSE_ROOT_ID]
		target.Name = fmt.Sprintf("overwrittenByRename%d", nextFreeInode)
		nextFreeInode++;

		self.inodePathMap[target.Key()] = target
	}

	self.inodePathMap[newKey] = data
}

func (self *PathFileSystemConnector) unlinkUpdate(nodeid uint64, name string) {
	self.lock.Lock()
	defer self.lock.Unlock()

	oldKey = inodeDataKey(nodeid, name)
	data := self.inodePathMap[oldKey]

	if data != nil {
		self.inodePathMap[oldKey] = nil, false
		self.inodePathMapByInode[data.NodeId] = nil, false
	}
}

////////////////////////////////////////////////////////////////
// Below routines should not access inodePathMap(ByInode) directly,
// and there need no locking.

func NewPathFileSystemConnector(fs PathFilesystem) (out *PathFileSystemConnector) {
	out = new(PathFileSystemConnector)
	out.inodePathMap = make(map[string]*inodeData)
	out.inodePathMapByInode = make(map[uint64]*inodeData)

	out.fileSystem = fs

	rootData := new(inodeData)
	rootData.Count = 1
	rootData.NodeId = FUSE_ROOT_ID

	out.inodePathMap[rootData.Key()] = rootData
	out.inodePathMapByInode[FUSE_ROOT_ID] = rootData
	out.nextFreeInode = FUSE_ROOT_ID + 1

	return out
}

func (self *PathFileSystemConnector) GetPath(nodeid uint64) string {
	return self.getInodeData(nodeid).GetPath()
}

func (self *PathFileSystemConnector) Init(h *InHeader, input *InitIn) (*InitOut, Status) {
	return self.fileSystem.Init()
}

func (self *PathFileSystemConnector) Destroy(h *InHeader, input *InitIn) {
	self.fileSystem.Destroy()
}

func (self *PathFileSystemConnector) Lookup(header *InHeader, name string) (out *EntryOut, status Status) {
	parent, ok := self.inodePathMapByInode[header.NodeId]
	if !ok {
		panic("Parent inode unknown.")
	}

	fullPath := path.Join(parent.GetPath(), name)
	attr, err := self.fileSystem.GetAttr(fullPath)
	if err != OK {
		return nil, err
	}

	data := lookupUpdate(header.NodeId, name)

	out = new(EntryOut)
	out.NodeId = data.NodeId
	out.Generation = 1 // where to get the generation?
	out.EntryValid = 0
	out.AttrValid = 0

	out.EntryValidNsec = 0
	out.AttrValidNsec = 0
	out.Attr = *attr

	return out, OK
}

func (self *PathFileSystemConnector) Forget(h *InHeader, input *ForgetIn) {
	self.forgetUpdate(h.NodeId, int(input.Nlookup))
}

func (self *PathFileSystemConnector) GetAttr(header *InHeader, input *GetAttrIn) (out *AttrOut, code Status) {
	attr, err := self.fileSystem.GetAttr(self.GetPath(header.NodeId))
	if err != OK {
		return nil, err
	}

	out = new(AttrOut)
	out.Attr = *attr

	// TODO - how to configure Valid timespans?
	out.AttrValid = 0

	// 0.1 second
	out.AttrValidNsec = 100e3

	return out, OK
}

func (self *PathFileSystemConnector) OpenDir(header *InHeader, input *OpenIn) (flags uint32, fuseFile RawFuseDir, status Status) {
	// TODO - how to handle return flags, the FUSE open flags?
	f, err := self.fileSystem.OpenDir(self.GetPath(header.NodeId))
	if err != OK {
		return 0, nil, err
	}

	return 0, f, OK
}

func (self *PathFileSystemConnector) Open(header *InHeader, input *OpenIn) (flags uint32, fuseFile RawFuseFile, status Status) {
	// TODO - how to handle return flags, the FUSE open flags?
	f, err := self.fileSystem.Open(self.GetPath(header.NodeId), input.Flags)
	if err != OK {
		return 0, nil, err
	}
	return 0, f, OK
}

func (self *PathFileSystemConnector) SetAttr(header *InHeader, input *SetAttrIn) (out *AttrOut, code Status) {
	var err Status = OK
	
	// TODO - support Fh.   (FSetAttr/FGetAttr/FTruncate.)
	fullPath := self.GetPath(header.NodeId)
	if input.Valid&FATTR_MODE != 0 {
		err = self.fileSystem.Chmod(fullPath, input.Mode)
	}
	if err != OK && (input.Valid&FATTR_UID != 0 || input.Valid&FATTR_GID != 0) {
		// TODO - can we get just FATTR_GID but not FATTR_UID ?
		err = self.fileSystem.Chown(fullPath, uint32(input.Uid), uint32(input.Gid))
	}
	if input.Valid&FATTR_SIZE != 0 {
		self.fileSystem.Truncate(fullPath, input.Size)
	}
	if err != OK && (input.Valid&FATTR_ATIME != 0 || input.Valid&FATTR_MTIME != 0) {
		err = self.fileSystem.Utimens(fullPath,
			uint64(input.Atime*1e9)+uint64(input.Atimensec),
			uint64(input.Mtime*1e9)+uint64(input.Mtimensec))
	}
	if err != OK && (input.Valid&FATTR_ATIME_NOW != 0 || input.Valid&FATTR_MTIME_NOW != 0) {
		// TODO - should set time to now. Maybe just reuse
		// Utimens() ?  Go has no UTIME_NOW unfortunately.
	}
	if err != OK {
		return nil, err
	}

	// TODO - where to get GetAttrIn.Flags / Fh ? 
	return self.GetAttr(header, new(GetAttrIn))
}

func (self *PathFileSystemConnector) Readlink(header *InHeader) (out []byte, code Status) {
	fullPath := self.GetPath(header.NodeId)
	val, err := self.fileSystem.Readlink(fullPath)
	return bytes.NewBufferString(val).Bytes(), err
}

func (self *PathFileSystemConnector) Mknod(header *InHeader, input *MknodIn, name string) (out *EntryOut, code Status) {
	fullPath := path.Join(self.GetPath(header.NodeId), name)
	err := self.fileSystem.Mknod(fullPath, input.Mode, uint32(input.Rdev))
	if err != OK {
		return nil, err
	}
	return self.Lookup(header, name)
}

func (self *PathFileSystemConnector) Mkdir(header *InHeader, input *MkdirIn, name string) (out *EntryOut, code Status) {
	err := self.fileSystem.Mkdir(path.Join(self.GetPath(header.NodeId), name), input.Mode)
	if err != OK {
		return nil, err
	}
	out, code = self.Lookup(header, name)
	return out, code
}

func (self *PathFileSystemConnector) Unlink(header *InHeader, name string) (code Status) {
	code = self.fileSystem.Unlink(path.Join(self.GetPath(header.NodeId), name))

	// Like fuse.c, we update our internal tables.
	self.unlinkUpdate(header.NodeId, name)
	
	return code
}

func (self *PathFileSystemConnector) Rmdir(header *InHeader, name string) (code Status) {
	code =  self.fileSystem.Rmdir(path.Join(self.GetPath(header.NodeId), name))
	self.unlinkUpdate(header.NodeId, name)
	return code
}

func (self *PathFileSystemConnector) Symlink(header *InHeader, pointedTo string, linkName string) (out *EntryOut, code Status) {
	err := self.fileSystem.Symlink(pointedTo, path.Join(self.GetPath(header.NodeId), linkName))
	if err != OK {
		return nil, err
	}

	out, code = self.Lookup(header, linkName)
	return out, code
}

func (self *PathFileSystemConnector) Rename(header *InHeader, input *RenameIn, oldName string, newName string) (code Status) {
	oldPath := path.Join(self.GetPath(header.NodeId), oldName)
	newPath := path.Join(self.GetPath(input.Newdir), newName)
	
	code = self.fileSystem.Rename(oldPath, newPath)
	if code != OK {
		return
	}
	
	// It is conceivable that the kernel module will issue a
	// forget for the old entry, and a lookup request for the new
	// one, but the fuse.c updates its client-side tables on its
	// own, so we do this as well.
	//
	// It should not hurt for us to do it here as well, although
	// it remains unclear how we should update Count.	
	self.renameUpdate(header.NodeId, oldName, input.Newdir, newName)
	return code
}

func (self *PathFileSystemConnector) Link(header *InHeader, input *LinkIn, filename string) (out *EntryOut, code Status) {
	orig := self.GetPath(input.Oldnodeid)
	newName := path.Join(self.GetPath(header.NodeId), filename)
	err := self.fileSystem.Link(orig, newName)

	if err != OK {
		return nil, err
	}

	return self.Lookup(header, filename)
}

func (self *PathFileSystemConnector) Access(header *InHeader, input *AccessIn) (code Status) {
	return self.fileSystem.Access(self.GetPath(header.NodeId), input.Mask)
}

func (self *PathFileSystemConnector) Create(header *InHeader, input *CreateIn, name string) (flags uint32, fuseFile RawFuseFile, out *EntryOut, code Status) {
	directory := self.GetPath(header.NodeId)
	fullPath := path.Join(directory, name)

	f, err := self.fileSystem.Create(fullPath, uint32(input.Flags), input.Mode)
	if err != OK {
		return 0, nil, nil, err
	}

	out, code = self.Lookup(header, name)
	return 0, f, out, code
}

////////////////////////////////////////////////////////////////
// unimplemented.

func (self *PathFileSystemConnector) SetXAttr(header *InHeader, input *SetXAttrIn) Status {
	return ENOSYS
}

func (self *PathFileSystemConnector) GetXAttr(header *InHeader, input *GetXAttrIn) (out *GetXAttrOut, code Status) {
	return nil, ENOSYS
}

func (self *PathFileSystemConnector) Bmap(header *InHeader, input *BmapIn) (out *BmapOut, code Status) {
	return nil, ENOSYS
}

func (self *PathFileSystemConnector) Ioctl(header *InHeader, input *IoctlIn) (out *IoctlOut, code Status) {
	return nil, ENOSYS
}

func (self *PathFileSystemConnector) Poll(header *InHeader, input *PollIn) (out *PollOut, code Status) {
	return nil, ENOSYS
}